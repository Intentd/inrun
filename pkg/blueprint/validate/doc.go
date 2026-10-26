// Package validate checks a Blueprint before anything runs. Execute runs every
// rule (structure, references, dependencies, serve and token configuration,
// gateway clusters) and returns the first failure. Nothing that fails
// validation is started.
package validate
