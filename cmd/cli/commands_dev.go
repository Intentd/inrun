//go:build !runtime && !gateway

package cli

import (
	_ "github.com/intentd/inrun/cmd/cli/cluster"
	_ "github.com/intentd/inrun/cmd/cli/console"
	_ "github.com/intentd/inrun/cmd/cli/gate"
	_ "github.com/intentd/inrun/cmd/cli/generate"
	_ "github.com/intentd/inrun/cmd/cli/migrate"
	_ "github.com/intentd/inrun/cmd/cli/notes"
	_ "github.com/intentd/inrun/cmd/cli/registry"
	_ "github.com/intentd/inrun/cmd/cli/rewrite"
	_ "github.com/intentd/inrun/cmd/cli/run"
	_ "github.com/intentd/inrun/cmd/cli/scaffold"
	_ "github.com/intentd/inrun/cmd/cli/self"
	_ "github.com/intentd/inrun/cmd/cli/serve"
	_ "github.com/intentd/inrun/cmd/cli/testsuite"
	_ "github.com/intentd/inrun/cmd/cli/validate"
)
