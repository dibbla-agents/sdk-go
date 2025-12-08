package jobs

// JobContext provides context for job execution
type JobContext struct {
	RunID   string
	JobID   string
	JobName string
	Args    map[string]interface{}
	Logger  *Logger
}

// GetArg retrieves an argument by name with type assertion
func (ctx *JobContext) GetArg(name string) (interface{}, bool) {
	if ctx.Args == nil {
		return nil, false
	}
	val, ok := ctx.Args[name]
	return val, ok
}

// GetStringArg retrieves a string argument
func (ctx *JobContext) GetStringArg(name string, defaultVal string) string {
	if val, ok := ctx.GetArg(name); ok {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return defaultVal
}

// GetIntArg retrieves an integer argument
func (ctx *JobContext) GetIntArg(name string, defaultVal int) int {
	if val, ok := ctx.GetArg(name); ok {
		switch v := val.(type) {
		case int:
			return v
		case float64:
			return int(v)
		}
	}
	return defaultVal
}

// GetBoolArg retrieves a boolean argument
func (ctx *JobContext) GetBoolArg(name string, defaultVal bool) bool {
	if val, ok := ctx.GetArg(name); ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return defaultVal
}

// GetFloat64Arg retrieves a float64 argument
func (ctx *JobContext) GetFloat64Arg(name string, defaultVal float64) float64 {
	if val, ok := ctx.GetArg(name); ok {
		switch v := val.(type) {
		case float64:
			return v
		case int:
			return float64(v)
		}
	}
	return defaultVal
}
