package kubectl

import (
	"context"
	"fmt"
)

// entry is one operation in a kubectl block: its position, such as
// "kubectl.get[0]", and the verb that runs it.
type entry struct {
	label string
	run   func(ctx context.Context) (string, error)
}

// reads lists the block's read entries in the order Assert runs them:
// get → logs → describe → exec → port-forward → events → auth → cp → top.
func reads(o BlockOptions) []entry {
	k, env, workDir, cs, dyn, mapper := o.Kubectl, o.Env, o.WorkDir, o.Client, o.Dynamic, o.Mapper
	var out []entry
	add := func(sub string, i int, run func(ctx context.Context) (string, error)) {
		out = append(out, entry{fmt.Sprintf("kubectl.%s[%d]", sub, i), run})
	}
	for i, e := range k.Get {
		add("get", i, func(ctx context.Context) (string, error) { return readGet(ctx, cs, dyn, mapper, e, env, workDir) })
	}
	for i, e := range k.Logs {
		add("logs", i, func(ctx context.Context) (string, error) { return readLogs(ctx, cs, e, env, workDir) })
	}
	for i, e := range k.Describe {
		add("describe", i, func(ctx context.Context) (string, error) { return readDescribe(ctx, e, env, workDir) })
	}
	for i, e := range k.Exec {
		add("exec", i, func(ctx context.Context) (string, error) { return readExec(ctx, cs, e, env, workDir) })
	}
	for i, e := range k.PortForward {
		add("port-forward", i, func(ctx context.Context) (string, error) {
			return readPortForward(ctx, cs, o.Config, e, env, workDir)
		})
	}
	for i, e := range k.Events {
		add("events", i, func(ctx context.Context) (string, error) { return readEvents(ctx, e, env, workDir) })
	}
	for i, e := range k.Auth {
		add("auth", i, func(ctx context.Context) (string, error) { return readAuth(ctx, cs, e) })
	}
	for i, e := range k.Cp {
		add("cp", i, func(ctx context.Context) (string, error) { return readCp(ctx, e, env, workDir) })
	}
	for i, e := range k.Top {
		add("top", i, func(ctx context.Context) (string, error) { return readTop(ctx, e, env, workDir) })
	}
	return out
}

// writes lists the block's write entries in the order Write runs them:
// apply → patch → restart → scale → delete.
func writes(o BlockOptions) []entry {
	k, env, workDir, cs := o.Kubectl, o.Env, o.WorkDir, o.Client
	var out []entry
	add := func(sub string, i int, run func(ctx context.Context) (string, error)) {
		out = append(out, entry{fmt.Sprintf("kubectl.%s[%d]", sub, i), run})
	}
	for i, e := range k.Apply {
		add("apply", i, func(ctx context.Context) (string, error) { return writeApply(ctx, e, env, workDir) })
	}
	for i, e := range k.Patch {
		add("patch", i, func(ctx context.Context) (string, error) { return writePatch(ctx, e, env, workDir) })
	}
	for i, e := range k.Restart {
		add("restart", i, func(ctx context.Context) (string, error) { return writeRestart(ctx, e, env, workDir) })
	}
	for i, e := range k.Scale {
		add("scale", i, func(ctx context.Context) (string, error) { return writeScale(ctx, e, env, workDir) })
	}
	for i, e := range k.Delete {
		add("delete", i, func(ctx context.Context) (string, error) { return writeDelete(ctx, cs, e, env, workDir) })
	}
	return out
}

// Assert runs the block's reads and their assertions, stopping at the first
// that fails.
func Assert(ctx context.Context, o BlockOptions) error {
	return runAll(ctx, reads(o))
}

// Write runs the block's writes, stopping at the first that fails. A step
// runs them once per attempt, before its assertions.
func Write(ctx context.Context, o BlockOptions) error {
	return runAll(ctx, writes(o))
}

// Post runs every entry in the block once, the writes and then the reads, and
// hands each one's label, output and error to report. Unlike a step it never
// stops early: a failed entry is reported and the next one runs, so a cleanup
// does all it can and a diagnostic prints all it can.
func Post(ctx context.Context, o BlockOptions, report func(label, out string, err error)) {
	for _, e := range append(writes(o), reads(o)...) {
		out, err := e.run(ctx)
		report(e.label, out, err)
	}
}

// runAll runs entries in order, returning the first error labelled with its
// position: "kubectl.get[0]: …".
func runAll(ctx context.Context, entries []entry) error {
	for _, e := range entries {
		if _, err := e.run(ctx); err != nil {
			return fmt.Errorf("%s: %w", e.label, err)
		}
	}
	return nil
}
