package object

func NewEnclosedEnvironment(outer *Environment) *Environment {
	env := NewEnvironment()
	env.outer = outer
	return env
}

func NewEnvironment() *Environment {
	s := make(map[string]Object)
	return &Environment{store: s, outer: nil}
}

type Environment struct {
	store map[string]Object
	outer *Environment
}

func (e *Environment) GetAll() map[string]Object {
	obj := e.store
	if e.outer != nil {
		obj = e.outer.store
	}
	return obj
}

func (e *Environment) Get(name string) (Object, bool) {
	obj, ok := e.store[name]
	if !ok && e.outer != nil {
		obj, ok = e.outer.Get(name)
	}
	return obj, ok
}

func (e *Environment) Set(name string, val Object) Object {
	e.store[name] = val
	return val
}

func (e *Environment) GlobalSet(name string, val Object) Object {
	if e.outer != nil {
		e.outer.store[name] = val
		return val
	}
	e.store[name] = val
	return val
}

func (e *Environment) Reset(name string) Object {
	delete(e.store, name)
	return &Null{}
}
