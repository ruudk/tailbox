package tailbox

import (
	"fmt"
	"io/ioutil"
	"math/big"
	"net/url"
	"os"
	"path"
	"syscall"
	"time"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

const WhenAlways = "always"
const WhenShutdown = "shutdown"
const DefaultLines = 6
const DefaultStopSignal = syscall.SIGINT
const DefaultStopTimeout = 10 * time.Second
const DefaultWaitStepInterval = 5 * time.Second
const DefaultWaitStepTimeout = 60 * time.Second

type Config struct {
	Version   int
	Defaults  DefaultsConfig
	Workflows []WorkflowConfig
}

type DefaultsConfig struct {
	Lines       int
	StopSignal  syscall.Signal
	StopTimeout time.Duration
}

type WorkflowConfig struct {
	Name  string
	When  string
	Steps []interface{}
}

type RunStepConfig struct {
	Name        string
	When        string
	Command     string
	Background  bool
	Lines       int
	StopSignal  syscall.Signal
	StopTimeout time.Duration
}

type WaitStepConfig struct {
	Name string

	Interval time.Duration
	Timeout  time.Duration
	URL      url.URL
}

type WatcherStepConfig struct {
	Name  string
	Globs []WatcherStepGlobConfig
	Steps []interface{}
}

type WatcherStepGlobConfig struct {
	Directory string
	Pattern   string
}

func NewConfig(filename string) (*Config, error) {
	config := Config{
		Defaults: DefaultsConfig{
			Lines:       DefaultLines,
			StopSignal:  DefaultStopSignal,
			StopTimeout: DefaultStopTimeout,
		},
	}
	err := decodeFile(filename, &config)
	if err != nil {
		return nil, fmt.Errorf("could not decode file: %w", err)
	}

	return &config, nil
}

func decodeFile(filename string, config *Config) error {
	ctx := &hcl.EvalContext{
		Variables: map[string]cty.Value{
			"directory": cty.StringVal(path.Dir(filename)),
		},
	}

	src, err := ioutil.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return hcl.Diagnostics{
				{
					Severity: hcl.DiagError,
					Summary:  "Configuration file not found",
					Detail:   fmt.Sprintf("The configuration file %s does not exist.", filename),
				},
			}
		}
		return hcl.Diagnostics{
			{
				Severity: hcl.DiagError,
				Summary:  "Failed to read configuration",
				Detail:   fmt.Sprintf("Can't read %s: %s.", filename, err),
			},
		}
	}

	file, diags := hclsyntax.ParseConfig(src, filename, hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return diags
	}

	schema := &hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{
			{
				Name:     "version",
				Required: true,
			},
		},
		Blocks: []hcl.BlockHeaderSchema{
			{
				Type: "defaults",
			},
			{
				Type:       "workflow",
				LabelNames: []string{"name"},
			},
		},
	}

	content, diags := file.Body.Content(schema)
	if diags.HasErrors() {
		return diags
	}

	for _, a := range content.Attributes {
		switch a.Name {
		case "version":
			config.Version, diags = asInt(ctx, a)
			if diags.HasErrors() {
				return diags
			}
		}
	}

	for _, b := range content.Blocks {
		switch b.Type {
		case "defaults":
			config.Defaults, diags = decodeDefaultBlock(ctx, config, b)
			if diags.HasErrors() {
				return diags
			}
		case "workflow":
			workflow, diags := decodeWorkflowBlock(ctx, config, b)
			if diags.HasErrors() {
				return diags
			}
			config.Workflows = append(config.Workflows, workflow)
		}
	}

	return nil
}

func decodeDefaultBlock(ctx *hcl.EvalContext, config *Config, block *hcl.Block) (DefaultsConfig, hcl.Diagnostics) {
	defaults := DefaultsConfig{
		Lines:       config.Defaults.Lines,
		StopSignal:  config.Defaults.StopSignal,
		StopTimeout: config.Defaults.StopTimeout,
	}

	schema := &hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{
			{
				Name: "lines",
			},
			{
				Name: "stop_signal",
			},
			{
				Name: "stop_timeout",
			},
		},
	}

	content, diags := block.Body.Content(schema)
	if diags.HasErrors() {
		return defaults, diags
	}

	for _, a := range content.Attributes {
		switch a.Name {
		case "lines":
			defaults.Lines, diags = asInt(ctx, a)
			if diags.HasErrors() {
				return defaults, diags
			}
		case "stop_signal":
			defaults.StopSignal, diags = decodeSignal(ctx, a)
			if diags.HasErrors() {
				return defaults, diags
			}
		case "stop_timeout":
			defaults.StopTimeout, diags = decodeDuration(ctx, a)
			if diags.HasErrors() {
				return defaults, diags
			}
		}
	}

	return defaults, diags
}

func decodeWorkflowBlock(ctx *hcl.EvalContext, config *Config, block *hcl.Block) (WorkflowConfig, hcl.Diagnostics) {
	workflow := WorkflowConfig{
		Name: block.Labels[0],
		When: WhenAlways,
	}

	schema := &hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{
			{
				Name: "when",
			},
		},
		Blocks: []hcl.BlockHeaderSchema{
			{
				Type:       "run",
				LabelNames: []string{"name"},
			},
			{
				Type:       "wait",
				LabelNames: []string{"name"},
			},
			{
				Type:       "watcher",
				LabelNames: []string{"name"},
			},
		},
	}

	content, diags := block.Body.Content(schema)
	if diags.HasErrors() {
		return workflow, diags
	}

	for _, a := range content.Attributes {
		switch a.Name {
		case "when":
			workflow.When, diags = decodeWhen(ctx, a)
			if diags.HasErrors() {
				return workflow, diags
			}
		}
	}

	for _, b := range content.Blocks {
		switch b.Type {
		case "run":
			step, diags := decodeRunStepConfig(ctx, config, b)
			if diags.HasErrors() {
				return workflow, diags
			}
			workflow.Steps = append(workflow.Steps, step)
		case "wait":
			step, diags := decodeWaitStepConfig(ctx, b)
			if diags.HasErrors() {
				return workflow, diags
			}
			workflow.Steps = append(workflow.Steps, step)
		case "watcher":
			step, diags := decodeWatcherStepConfig(ctx, config, b)
			if diags.HasErrors() {
				return workflow, diags
			}
			workflow.Steps = append(workflow.Steps, step)
		}
	}

	return workflow, diags
}

func decodeRunStepConfig(ctx *hcl.EvalContext, config *Config, block *hcl.Block) (RunStepConfig, hcl.Diagnostics) {
	step := RunStepConfig{
		Name:        block.Labels[0],
		Background:  false,
		Lines:       config.Defaults.Lines,
		StopSignal:  config.Defaults.StopSignal,
		StopTimeout: config.Defaults.StopTimeout,
	}

	schema := &hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{
			{
				Name: "when",
			},
			{
				Name:     "command",
				Required: true,
			},
			{
				Name: "background",
			},
			{
				Name: "lines",
			},
			{
				Name: "stop_signal",
			},
			{
				Name: "stop_timeout",
			},
		},
	}

	content, diags := block.Body.Content(schema)
	if diags.HasErrors() {
		return step, diags
	}

	for _, a := range content.Attributes {
		switch a.Name {
		case "when":
			// TODO What is allowed here? Only empty and always??
			step.When, diags = decodeWhen(ctx, a)
			if diags.HasErrors() {
				return step, diags
			}
		case "command":
			c, diags := a.Expr.Value(ctx)
			if diags.HasErrors() {
				return step, diags
			}
			step.Command = c.AsString()
		case "background":
			v, diags := a.Expr.Value(ctx)
			if diags.HasErrors() {
				return step, diags
			}
			step.Background = v.True()
		case "lines":
			step.Lines, diags = asInt(ctx, a)
			if diags.HasErrors() {
				return step, diags
			}
		case "stop_signal":
			step.StopSignal, diags = decodeSignal(ctx, a)
			if diags.HasErrors() {
				return step, diags
			}
		case "stop_timeout":
			step.StopTimeout, diags = decodeDuration(ctx, a)
			if diags.HasErrors() {
				return step, diags
			}
		}
	}

	return step, diags
}

func decodeWaitStepConfig(ctx *hcl.EvalContext, block *hcl.Block) (WaitStepConfig, hcl.Diagnostics) {
	step := WaitStepConfig{
		Name:     block.Labels[0],
		Interval: DefaultWaitStepInterval,
		Timeout:  DefaultWaitStepTimeout,
	}

	schema := &hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{
			{
				Name: "interval",
			},
			{
				Name: "timeout",
			},
			{
				Name:     "url",
				Required: true,
			},
		},
	}

	content, diags := block.Body.Content(schema)
	if diags.HasErrors() {
		return step, diags
	}

	for _, a := range content.Attributes {
		switch a.Name {
		case "interval":
			step.Interval, diags = decodeDuration(ctx, a)
			if diags.HasErrors() {
				return step, diags
			}
		case "timeout":
			step.Timeout, diags = decodeDuration(ctx, a)
			if diags.HasErrors() {
				return step, diags
			}
		case "url":
			step.URL, diags = decodeUrl(ctx, a)
			if diags.HasErrors() {
				return WaitStepConfig{}, diags
			}
		}
	}

	return step, diags
}

func decodeWatcherStepConfig(ctx *hcl.EvalContext, config *Config, block *hcl.Block) (WatcherStepConfig, hcl.Diagnostics) {
	step := WatcherStepConfig{
		Name: block.Labels[0],
	}

	schema := &hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{
				Type: "glob",
			},
			{
				Type:       "run",
				LabelNames: []string{"name"},
			},
			{
				Type:       "wait",
				LabelNames: []string{"name"},
			},
		},
	}

	content, diags := block.Body.Content(schema)
	if diags.HasErrors() {
		return step, diags
	}

	for _, b := range content.Blocks {
		switch b.Type {
		case "glob":
			glob, diags := decodeWatcherStepGlobConfig(ctx, b)
			if diags.HasErrors() {
				return step, diags
			}
			step.Globs = append(step.Globs, glob)
		case "run":
			subStep, diags := decodeRunStepConfig(ctx, config, b)
			if diags.HasErrors() {
				return step, diags
			}
			step.Steps = append(step.Steps, subStep)
		case "wait":
			subStep, diags := decodeWaitStepConfig(ctx, b)
			if diags.HasErrors() {
				return step, diags
			}
			step.Steps = append(step.Steps, subStep)
		}
	}

	return step, diags
}

func decodeWatcherStepGlobConfig(ctx *hcl.EvalContext, block *hcl.Block) (WatcherStepGlobConfig, hcl.Diagnostics) {
	var glob WatcherStepGlobConfig

	schema := &hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{
			{
				Name:     "directory",
				Required: true,
			},
			{
				Name:     "pattern",
				Required: true,
			},
		},
	}

	content, diags := block.Body.Content(schema)
	if diags.HasErrors() {
		return glob, diags
	}

	for _, a := range content.Attributes {
		switch a.Name {
		case "directory":
			v, diags := a.Expr.Value(ctx)
			if diags.HasErrors() {
				return glob, diags
			}
			glob.Directory = v.AsString()
		case "pattern":
			v, diags := a.Expr.Value(ctx)
			if diags.HasErrors() {
				return glob, diags
			}
			glob.Pattern = v.AsString()
		}
	}

	return glob, diags
}

func decodeUrl(ctx *hcl.EvalContext, attr *hcl.Attribute) (url.URL, hcl.Diagnostics) {
	v, diags := attr.Expr.Value(ctx)
	if diags.HasErrors() {
		return url.URL{}, diags
	}

	u, err := url.Parse(v.AsString())
	if err != nil {
		r := attr.Range
		return url.URL{}, hcl.Diagnostics{
			{
				Subject:  &r,
				Severity: hcl.DiagError,
				Summary:  fmt.Sprintf("URL %s is not valid: %s", v.AsString(), err),
			},
		}
	}

	return *u, nil
}

func decodeSignal(ctx *hcl.EvalContext, attr *hcl.Attribute) (syscall.Signal, hcl.Diagnostics) {
	v, diags := attr.Expr.Value(ctx)
	if diags.HasErrors() {
		return syscall.Signal(0), diags
	}

	switch v.AsString() {
	case "int":
		return syscall.SIGINT, nil
	default:
		r := attr.Range
		return 0, hcl.Diagnostics{
			{
				Subject:  &r,
				Severity: hcl.DiagError,
				Summary:  fmt.Sprintf("Invalid signal %s.", v.AsString()),
			},
		}
	}
}

func decodeWhen(ctx *hcl.EvalContext, attr *hcl.Attribute) (string, hcl.Diagnostics) {
	v, diags := attr.Expr.Value(ctx)
	if diags.HasErrors() {
		return "", diags
	}

	switch v.AsString() {
	case WhenAlways:
		return WhenAlways, nil
	case WhenShutdown:
		return WhenShutdown, nil
	default:
		r := attr.Range
		return "", hcl.Diagnostics{
			{
				Subject:  &r,
				Severity: hcl.DiagError,
				Summary:  fmt.Sprintf("Invalid option %s.", v.AsString()),
			},
		}
	}
}

func decodeDuration(ctx *hcl.EvalContext, attr *hcl.Attribute) (time.Duration, hcl.Diagnostics) {
	v, diags := asInt(ctx, attr)
	if diags.HasErrors() {
		return time.Duration(0), diags
	}

	return time.Duration(v) * time.Second, nil
}

func decodeStringSlice(ctx *hcl.EvalContext, attr *hcl.Attribute) ([]string, hcl.Diagnostics) {
	v, diags := attr.Expr.Value(ctx)
	if diags.HasErrors() {
		return nil, diags
	}

	in := v.AsValueSlice()
	out := make([]string, len(in))
	for i, v := range in {
		out[i] = v.AsString()
	}

	return out, diags
}

func asInt(ctx *hcl.EvalContext, attr *hcl.Attribute) (int, hcl.Diagnostics) {
	v, diags := attr.Expr.Value(ctx)
	if diags.HasErrors() {
		return 0, diags
	}

	if !v.AsBigFloat().IsInt() {
		r := attr.Range
		return 0, hcl.Diagnostics{
			{
				Subject:  &r,
				Severity: hcl.DiagError,
				Summary:  fmt.Sprintf("Field %s should be int.", attr.Name),
			},
		}
	}

	bi, accuracy := v.AsBigFloat().Int64()
	if accuracy != big.Exact {
		r := attr.Range
		return 0, hcl.Diagnostics{
			{
				Subject:  &r,
				Severity: hcl.DiagError,
				Summary:  fmt.Sprintf("Field %s value could not be converted to int.", attr.Name),
			},
		}
	}

	return int(bi), hcl.Diagnostics{}
}
