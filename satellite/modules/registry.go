package modules

import "go.uber.org/dig"

// Registry is the only one holy registry for all provided source.
var Registry *dig.Container = dig.New()
