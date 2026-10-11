package admission

import (
	"github.com/intentd/inrun/pkg/catalog"
	"github.com/intentd/inrun/pkg/catalog/validate/internal/check"
	"github.com/intentd/inrun/pkg/types"
)

// newExec returns a checker over cat.
func newExec(cat *catalog.Catalog) *checker { return &checker{check.New(cat, nil)} }

// newCatalogExec returns a checker over a test Catalog holding crds.
func newCatalogExec(crds map[string]types.CRDEntry) *checker {
	return newExec(catalog.NewCatalogForTest(crds))
}
