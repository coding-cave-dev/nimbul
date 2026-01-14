package k8s

import (
	"context"
	"fmt"
	"os"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
)

func getConfig() (*rest.Config, error) {
	kubeConfigPath := os.Getenv("KUBECONFIG")
	if kubeConfigPath == "" {
		config, err := rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
		return config, nil
	}

	// Use the kubeconfig path directly - no need for flags
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		return nil, err
	}

	return config, nil
}

func GetClient() (*kubernetes.Clientset, error) {
	config, err := getConfig()
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(config)
}

// GetDynamicClient returns a dynamic client for applying arbitrary Kubernetes resources
func GetDynamicClient() (dynamic.Interface, error) {
	config, err := getConfig()
	if err != nil {
		return nil, err
	}
	return dynamic.NewForConfig(config)
}

// ApplyManifests applies multi-document YAML manifests to the cluster
func ApplyManifests(ctx context.Context, yamlBytes []byte) error {
	// Get dynamic client
	dynamicClient, err := GetDynamicClient()
	if err != nil {
		return fmt.Errorf("failed to get dynamic client: %w", err)
	}

	// Get REST config for discovery
	config, err := getConfig()
	if err != nil {
		return fmt.Errorf("failed to get config: %w", err)
	}

	// Create discovery client and REST mapper
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create discovery client: %w", err)
	}

	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(discoveryClient))

	// Create YAML decoder
	decoder := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)

	// Split multi-document YAML
	manifests := strings.Split(string(yamlBytes), "---")
	if len(manifests) == 0 {
		return fmt.Errorf("no manifests found")
	}

	// Apply each manifest
	for i, manifest := range manifests {
		manifest = strings.TrimSpace(manifest)
		if manifest == "" {
			continue
		}

		// Decode YAML into unstructured object
		obj := &unstructured.Unstructured{}
		_, gvk, err := decoder.Decode([]byte(manifest), nil, obj)
		if err != nil {
			return fmt.Errorf("failed to decode manifest %d: %w", i+1, err)
		}

		// Find GVR (GroupVersionResource) from GVK (GroupVersionKind)
		mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			return fmt.Errorf("failed to find REST mapping for %s: %w", gvk, err)
		}

		// Get resource interface
		var dr dynamic.ResourceInterface
		if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
			// Namespaced resource
			namespace := obj.GetNamespace()
			if namespace == "" {
				namespace = "default"
			}
			dr = dynamicClient.Resource(mapping.Resource).Namespace(namespace)
		} else {
			// Cluster-scoped resource
			dr = dynamicClient.Resource(mapping.Resource)
		}

		// Get resource name
		name := obj.GetName()
		if name == "" {
			return fmt.Errorf("manifest %d: resource name is required", i+1)
		}

		// Apply using server-side apply
		_, err = dr.Apply(ctx, name, obj, metav1.ApplyOptions{
			FieldManager: "nimbul",
			Force:        true,
		})
		if err != nil {
			return fmt.Errorf("failed to apply resource %s/%s (%s): %w", obj.GetNamespace(), name, gvk, err)
		}

		fmt.Printf("✓ Applied %s %s/%s\n", gvk.Kind, obj.GetNamespace(), name)
	}

	return nil
}
