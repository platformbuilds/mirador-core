package utils

import "errors"

// TOON conversion utilities were moved to the external mira-service.
var ErrTOONUnavailable = errors.New("TOON conversion removed from core; use external mira-service")

func ConvertRCAToTOON(_ interface{}) (string, error)     { return "", ErrTOONUnavailable }
func ConvertRCADataToTOON(_ interface{}) (string, error) { return "", ErrTOONUnavailable }
func ValidateRCAResponse(_ interface{}) error            { return ErrTOONUnavailable }
