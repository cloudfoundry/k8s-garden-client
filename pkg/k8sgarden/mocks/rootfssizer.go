package mocks

import "code.cloudfoundry.org/executor/initializer/configuration"

type mockRootFSSizer struct {
}

var _ configuration.RootFSSizer = &mockRootFSSizer{}

func NewRootFSSizer() configuration.RootFSSizer {
	return &mockRootFSSizer{}
}

func (m *mockRootFSSizer) RootFSSizeFromPath(path string) uint64 {
	return 0
}
