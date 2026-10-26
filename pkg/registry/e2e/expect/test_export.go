package expect

import "github.com/intentd/inrun/pkg/types"

func Call(name, url string) types.E2EExternalCheck {
	c := types.E2EExternalCheck{}
	c.Name, c.URL = name, url
	return c
}
