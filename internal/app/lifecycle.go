package app

import "context"

func (s *State) Startup(ctx context.Context) {
	if s.Runtime != nil {
		s.Runtime.Startup(ctx)
	}
}

func (s *State) Shutdown() {
	if s.Runtime != nil {
		s.Runtime.Shutdown()
	}
}

func (s *State) EmitEvent(name string, data interface{}) {
	if s.Runtime != nil {
		s.Runtime.EmitEvent(s.Ctx, name, data)
	}
}
