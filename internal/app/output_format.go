package app

import (
	"encoding"
	"fmt"
)

var _ encoding.TextUnmarshaler = (*outputFormat)(nil)

type outputFormat string

const (
	outputFormatText outputFormat = "text"
	outputFormatJSON outputFormat = "json"
)

func (format *outputFormat) UnmarshalText(text []byte) error {
	if format == nil {
		return nil
	}

	value := outputFormat(text)
	switch value {
	case outputFormatText, outputFormatJSON:
		*format = value
		return nil
	default:
		return fmt.Errorf("invalid output format %q", string(text))
	}
}
