package mobileapi

import "github.com/axuitomo/CFST-GUI/internal/kernel/domain/probe/colodict"

func (s *Service) coloDictionaryPaths() colodict.Paths {
	return colodict.DefaultPaths(s.basePath())
}
