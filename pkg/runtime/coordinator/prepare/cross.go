package prepare

import (
	"context"
	"fmt"

	"github.com/intentd/inrun/pkg/runtimeapi"

	"github.com/intentd/inrun/pkg/util/structured"

	"github.com/intentd/inrun/domain"
	"github.com/intentd/inrun/pkg/logger"
	"github.com/intentd/inrun/pkg/template"
	"github.com/intentd/inrun/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// CatalogRegistry is the minimal read interface prepare needs for cross-CRD
// observation. *coordinator.ResourceCatalog satisfies this.
type CatalogRegistry interface {
	GetInformerByName(name string) (cache.SharedIndexInformer, bool)
	GetInformerByLabelSelector(key, value string) (cache.SharedIndexInformer, bool)
	GetCrossAccessByName(name string) *bool
}

// readCross reads cross-CRD observations for all declared cross: entries.
// Returns the map injected via resolver.WithCross().
//
// Resolution priority per declaration:
//  1. Informer cache via registry — zero API calls, same-binary CRDs
//  2. HTTP endpoint — cross-binary or cross-cluster
//  3. Not-found result
func readCross(
	ctx context.Context,
	obj domain.Object,
	decls []types.CrossCRDDeclaration,
	resolver *template.Resolver,
	registry CatalogRegistry,
	cs kubernetes.Interface,
) map[string]interface{} {
	if len(decls) == 0 {
		return nil
	}

	log := logger.FromContext(ctx)
	result := make(map[string]interface{}, len(decls))

	for _, decl := range decls {
		as := decl.As
		if as == "" {
			as = decl.CRD
		}

		name, _ := resolver.Resolve(decl.Selector.Name)
		namespace, _ := resolver.Resolve(decl.Selector.Namespace)
		if namespace == "" {
			namespace = obj.GetNamespace()
		}

		var inf cache.SharedIndexInformer
		if registry != nil {
			switch {
			case decl.IsCRDBased():
				if i, found := registry.GetInformerByName(decl.CRD); found {
					inf = i
				}
			case decl.IsLabelBased():
				for k, v := range decl.LabelSelector {
					if i, found := registry.GetInformerByLabelSelector(k, v); found {
						inf = i
						break
					}
				}
			}
		}

		if inf != nil {
			var crossAccess *bool
			if decl.IsCRDBased() {
				crossAccess = registry.GetCrossAccessByName(decl.CRD)
			}

			sel := decl.Selector
			var data map[string]interface{}
			switch {
			case !sel.MatchLabels.Empty():
				for k, v := range sel.MatchLabels {
					data = readCrossFromInformerByLabel(inf.GetIndexer(), k, v)
					break
				}
			case decl.IsLabelBased():
				for k, v := range decl.LabelSelector {
					data = readCrossFromInformerByLabel(inf.GetIndexer(), k, v)
					break
				}
			case sel.IsNameBased():
				data = readCrossFromInformerByName(inf.GetIndexer(), crossKey(namespace, name), crossAccess)
			}

			if data != nil {
				result[as] = data
				log.Debug().Str("crd", decl.CRD).Str("as", as).Msg("cross: read from informer cache")
				continue
			}
			log.Warn().Str("crd", decl.CRD).Str("as", as).Msg("cross: informer found but CR not matched")
		} else if registry != nil {
			log.Warn().Str("crd", decl.CRD).Str("as", as).Msg("cross: CRD not found in registry — trying HTTP")
		}

		// HTTP fallback
		if decl.HasSource() {
			if decl.Source.HasEndpoint() {
				src := *decl.Source
				src.Endpoint, _ = resolver.Resolve(decl.Source.Endpoint)
				_, data := runtimeapi.FetchCrossViaHTTP(ctx, cs, &src)
				if data != nil {
					result[as] = data
					log.Debug().Str("crd", decl.CRD).Str("as", as).Str("endpoint", src.Endpoint).Msg("cross: read via raw endpoint")
					continue
				}
				log.Warn().Str("crd", decl.CRD).Str("endpoint", src.Endpoint).Msg("cross: raw endpoint returned nil")
			}
			if decl.Source.HasHost() {
				src := *decl.Source
				src.Endpoint = types.BuildONCOPURL(decl)
				_, data := runtimeapi.FetchCrossViaHTTP(ctx, cs, &src)
				if data != nil {
					result[as] = data
					log.Debug().Str("crd", decl.CRD).Str("as", as).Str("endpoint", src.Endpoint).Msg("cross: read via ONCOP host")
					continue
				}
				log.Warn().Str("crd", decl.CRD).Str("endpoint", src.Endpoint).Msg("cross: ONCOP endpoint returned nil")
			}
		}

		result[as] = map[string]interface{}{
			"found":     "false",
			"name":      name,
			"namespace": namespace,
			"status":    map[string]interface{}{},
			"spec":      map[string]interface{}{},
		}
		log.Debug().Str("crd", decl.CRD).Str("as", as).Msg("cross: not found — empty result")
	}

	return result
}

func readCrossFromInformerByName(indexer cache.Indexer, key string, crossAccess *bool) map[string]interface{} {
	if crossAccess != nil && !*crossAccess {
		return notFoundCrossResult()
	}
	raw, exists, err := indexer.GetByKey(key)
	if err != nil || !exists {
		return notFoundCrossResult()
	}
	objMap, err := structured.RawToMap(raw)
	if err != nil {
		return notFoundCrossResult()
	}
	return buildCrossResultFromMap(objMap)
}

func readCrossFromInformerByLabel(indexer cache.Indexer, labelKey, labelValue string) map[string]interface{} {
	for _, raw := range indexer.List() {
		objMap, err := structured.RawToMap(raw)
		if err != nil {
			continue
		}
		labels, _ := objMap["metadata"].(map[string]interface{})["labels"].(map[string]interface{})
		if fmt.Sprint(labels[labelKey]) == labelValue {
			return buildCrossResultFromMap(objMap)
		}
	}
	return notFoundCrossResult()
}

func buildCrossResultFromMap(objMap map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, 6)
	result["found"] = "true"
	result["name"] = structured.MetaField(objMap, "name")
	result["namespace"] = structured.MetaField(objMap, "namespace")

	if spec, ok := objMap["spec"].(map[string]interface{}); ok && spec != nil {
		result["spec"] = spec
	} else {
		result["spec"] = map[string]interface{}{}
	}
	if status, ok := objMap["status"].(map[string]interface{}); ok && status != nil {
		result["status"] = status
	} else {
		result["status"] = map[string]interface{}{}
	}
	meta, _ := objMap["metadata"].(map[string]interface{})
	if rawLabels, ok := meta["labels"].(map[string]interface{}); ok && len(rawLabels) > 0 {
		result["labels"] = rawLabels
	}
	if rawAnnotations, ok := meta["annotations"].(map[string]interface{}); ok && len(rawAnnotations) > 0 {
		result["annotations"] = rawAnnotations
	}
	return result
}

func notFoundCrossResult() map[string]interface{} {
	return map[string]interface{}{
		"found":  "false",
		"spec":   map[string]interface{}{},
		"status": map[string]interface{}{},
	}
}

func crossKey(namespace, name string) string {
	if namespace == "" {
		return name
	}
	return fmt.Sprintf("%s/%s", namespace, name)
}
