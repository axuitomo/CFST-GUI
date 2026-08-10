package mobileapi

import "github.com/axuitomo/CFST-GUI/internal/colodict"

func (s *Service) coloDictionaryPaths() colodict.Paths {
	return colodict.DefaultPaths(s.basePath())
}
