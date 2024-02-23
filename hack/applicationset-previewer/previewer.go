package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubefake "k8s.io/client-go/kubernetes/fake"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	k8scache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/yaml"

	"github.com/argoproj/argo-cd/v2/applicationset/controllers"
	"github.com/argoproj/argo-cd/v2/applicationset/generators"
	"github.com/argoproj/argo-cd/v2/applicationset/utils"
	appv1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned/fake"
	"github.com/argoproj/argo-cd/v2/util/config"
	dbmocks "github.com/argoproj/argo-cd/v2/util/db/mocks"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
)

func Generate(filePaths []string) ([]appv1alpha1.Application, error) {
	objects, err := parseYAMLs(filePaths)
	if err != nil {
		return nil, fmt.Errorf("parsing yaml: %w", err)
	}

	cntr, err := newController(context.Background(), objects)
	if err != nil {
		return nil, fmt.Errorf("creating controller: %w", err)
	}

	var res []appv1alpha1.Application
	asets := filterAppSets(objects)

	log.Infof("Processing %d ApplicationSets and %d secrets", len(asets), len(objects)-len(asets))

	for _, as := range asets {
		lg := log.StandardLogger()
		lg.SetLevel(log.ErrorLevel)
		apps, _, err := cntr.GenerateApplications(log.NewEntry(lg), *as)
		if err != nil {
			return nil, err
		}
		for i := range apps {
			fixupApp(&apps[i])
		}
		res = append(res, apps...)
	}

	// Sort the results for reproducibility.
	sort.Slice(res, func(i, j int) bool {
		if res[i].Namespace != res[j].Namespace {
			return res[i].Namespace < res[j].Namespace
		}
		return res[i].Name < res[j].Name
	})

	return res, nil
}

func DumpApps(w io.Writer, apps []appv1alpha1.Application) {
	for _, as := range apps {
		data, _ := yaml.Marshal(as)
		w.Write([]byte("---\n"))
		w.Write(data)
	}
}

func filterAppSets(objs []client.Object) []*appv1alpha1.ApplicationSet {
	var res []*appv1alpha1.ApplicationSet
	for _, obj := range objs {
		if obj.GetObjectKind().GroupVersionKind().Kind == "ApplicationSet" {
			res = append(res, obj.(*appv1alpha1.ApplicationSet))
		}
	}
	return res
}

func newController(ctx context.Context, objs []client.Object) (*controllers.ApplicationSetReconciler, error) {
	// See https://github.com/argoproj/argo-cd/blob/df2b0e271111f41e8fdfb97e2b4e19b9e623706b/cmd/argocd-applicationset-controller/commands/applicationset_controller.go#L45.
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)

	namespace := "argocd"

	srObjs := toRuntimeObjects(filterKind(objs, "Secret"))
	arObjs := toRuntimeObjects(filterKind(objs, "ApplicationSet"))

	cclient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).WithIndex(&appv1alpha1.Application{}, ".metadata.controller", appControllerIndexer).Build()
	fclientSet := kubefake.NewSimpleClientset(srObjs...)

	argoDBMock := dbmocks.ArgoDB{}

	terminalGenerators := map[string]generators.Generator{
		"List":     generators.NewListGenerator(),
		"Clusters": generators.NewClusterGenerator(cclient, ctx, fclientSet, namespace),
		// "Git":                     generators.NewGitGenerator(argoCDService),
		// "SCMProvider":             generators.NewSCMProviderGenerator(mgr.GetClient(), scmAuth, scmRootCAPath, allowedScmProviders, enableScmProviders),
		// "ClusterDecisionResource": generators.NewDuckTypeGenerator(ctx, dynamicClient, k8sClient, namespace),
		// "PullRequest":             generators.NewPullRequestGenerator(mgr.GetClient(), scmAuth, scmRootCAPath, allowedScmProviders, enableScmProviders),
		// "Plugin":                  generators.NewPluginGenerator(mgr.GetClient(), ctx, k8sClient, namespace),
	}

	nestedGenerators := map[string]generators.Generator{
		"List":     terminalGenerators["List"],
		"Clusters": terminalGenerators["Clusters"],
		// "Git":                     terminalGenerators["Git"],
		// "SCMProvider":             terminalGenerators["SCMProvider"],
		// "ClusterDecisionResource": terminalGenerators["ClusterDecisionResource"],
		// "PullRequest":             terminalGenerators["PullRequest"],
		// "Plugin":                  terminalGenerators["Plugin"],
		"Matrix": generators.NewMatrixGenerator(terminalGenerators),
		"Merge":  generators.NewMergeGenerator(terminalGenerators),
	}

	topLevelGenerators := map[string]generators.Generator{
		"List":     terminalGenerators["List"],
		"Clusters": terminalGenerators["Clusters"],
		//"Git":                     terminalGenerators["Git"],
		//"SCMProvider":             terminalGenerators["SCMProvider"],
		//"ClusterDecisionResource": terminalGenerators["ClusterDecisionResource"],
		//"PullRequest":             terminalGenerators["PullRequest"],
		//"Plugin":                  terminalGenerators["Plugin"],
		"Matrix": generators.NewMatrixGenerator(nestedGenerators),
		"Merge":  generators.NewMergeGenerator(nestedGenerators),
	}
	res := &controllers.ApplicationSetReconciler{
		Client:           cclient,
		Scheme:           scheme,
		Renderer:         &utils.Render{},
		Recorder:         record.NewFakeRecorder(10),
		Cache:            &fakeCache{},
		Generators:       topLevelGenerators,
		ArgoDB:           &argoDBMock,
		ArgoCDNamespace:  namespace,
		ArgoAppClientset: appclientset.NewSimpleClientset(arObjs...),
	}
	return res, nil
}

func appControllerIndexer(rawObj client.Object) []string {
	// grab the job object, extract the owner...
	app := rawObj.(*appv1alpha1.Application)
	owner := metav1.GetControllerOf(app)
	if owner == nil {
		return nil
	}
	// ...make sure it's a application set...
	if owner.APIVersion != appv1alpha1.SchemeGroupVersion.String() || owner.Kind != "ApplicationSet" {
		return nil
	}

	// ...and if so, return it
	return []string{owner.Name}
}

func parseYAMLs(yamlPaths []string) ([]client.Object, error) {
	var (
		res      []client.Object
		contents [][]byte
	)
	for _, path := range yamlPaths {
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading yaml: %w", err)
		}
		contents = append(contents, b)
	}
	if len(contents) == 0 {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("reading yaml from stdin: %w", err)
		}
		contents = append(contents, b)
		// For debugging.
		yamlPaths = append(yamlPaths, "stdin")
	}

	for i, b := range contents {
		obj, err := parseYAML(b, yamlPaths[i])
		if err != nil {
			return nil, fmt.Errorf("parsing yaml: %w", err)
		}
		res = append(res, obj...)
	}

	return res, nil
}

func parseYAML(yml []byte, fname string) ([]client.Object, error) {
	// Only secrets and ApplicationSets are supported.
	yamls, err := kube.SplitYAMLToString(yml)
	if err != nil {
		return nil, fmt.Errorf("splitting YAML to string: %w", err)
	}

	var res []client.Object

	for _, yml := range yamls {
		// Determine which object we're dealing with.
		var meta metav1.TypeMeta
		if err := config.Unmarshal([]byte(yml), &meta); err != nil {
			return nil, fmt.Errorf("%s: unmarshalling type meta: %w", fname, err)
		}
		var obj client.Object
		switch meta.Kind {
		case "Secret":
			obj = &corev1.Secret{}
		case "ApplicationSet":
			obj = &appv1alpha1.ApplicationSet{}
		default:
			log.Warnf("Path: %s, Ignored unsupported kind: %s", fname, meta.Kind)
			continue
		}
		// Parse the object with the right type.
		if err := config.Unmarshal([]byte(yml), obj); err != nil {
			return nil, fmt.Errorf("%s: unmarshalling type meta: %w", fname, err)
		}
		if meta.Kind == "Secret" {
			if err := fixupSecret(obj.(*corev1.Secret)); err != nil {
				return nil, fmt.Errorf("%s: decoding secret: %w", fname, err)
			}
		}
		res = append(res, obj)
	}

	return res, nil
}

func fixupSecret(secret *corev1.Secret) error {
	// Decode possibly encoded data.
	if secret.Data == nil {
		secret.Data = map[string][]byte{}
	}
	// Move string data to data.
	for k, v := range secret.StringData {
		secret.Data[k] = []byte(v)
	}
	return nil
}

func fixupApp(app *appv1alpha1.Application) {
	app.APIVersion = appv1alpha1.SchemeGroupVersion.String()
	app.Kind = appv1alpha1.ApplicationSchemaGroupVersionKind.Kind
}

func filterKind(objs []client.Object, kind string) []client.Object {
	res := []client.Object{}
	for _, obj := range objs {
		if obj.GetObjectKind().GroupVersionKind().Kind == kind {
			res = append(res, obj)
		}
	}
	return res
}

func toRuntimeObjects(objs []client.Object) []runtime.Object {
	res := []runtime.Object{}
	for _, clientCluster := range objs {
		res = append(res, clientCluster)
	}
	return res
}

type fakeStore struct {
	k8scache.Store
}

func (f *fakeStore) Update(obj interface{}) error {
	return nil
}

type fakeInformer struct {
	k8scache.SharedInformer
}

func (f *fakeInformer) AddIndexers(indexers k8scache.Indexers) error {
	return nil
}

func (f *fakeInformer) GetStore() k8scache.Store {
	return &fakeStore{}
}

type fakeCache struct {
	cache.Cache
}

func (f *fakeCache) GetInformer(ctx context.Context, obj client.Object) (cache.Informer, error) {
	return &fakeInformer{}, nil
}
