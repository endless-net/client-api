package clientapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

const (
	maxPublicErrorBytes          = 16 << 10
	maxRegisterNodeRequestBytes  = 64 << 10
	maxRegisterNodeResponseBytes = 4 << 20
)

// MarshalPublicError validates and serializes a canonical v2 public error.
func MarshalPublicError(value PublicError) ([]byte, error) {
	return marshalValidated(value, value.Validate)
}

// DecodePublicError strictly decodes a single v2 public error JSON object.
func DecodePublicError(reader io.Reader) (PublicError, error) {
	var value PublicError
	err := decodeStrict(reader, maxPublicErrorBytes, &value)
	if err == nil {
		err = value.Validate()
	}
	return value, err
}

// MarshalRegisterNodeRequest validates and serializes a canonical v2 request.
func MarshalRegisterNodeRequest(value RegisterNodeRequest) ([]byte, error) {
	return marshalValidated(value, value.Validate)
}

// DecodeRegisterNodeRequest strictly decodes one v2 registration request.
func DecodeRegisterNodeRequest(reader io.Reader) (RegisterNodeRequest, error) {
	var value RegisterNodeRequest
	err := decodeStrict(reader, maxRegisterNodeRequestBytes, &value)
	if err == nil {
		err = value.Validate()
	}
	return value, err
}

// MarshalRegisterNodeResponse validates and serializes a canonical v2 result.
func MarshalRegisterNodeResponse(value RegisterNodeResponse) ([]byte, error) {
	return marshalValidated(value, value.Validate)
}

// DecodeRegisterNodeResponse strictly decodes one v2 registration result.
func DecodeRegisterNodeResponse(reader io.Reader) (RegisterNodeResponse, error) {
	var value RegisterNodeResponse
	err := decodeStrict(reader, maxRegisterNodeResponseBytes, &value)
	if err == nil {
		err = value.Validate()
	}
	return value, err
}

func marshalValidated(value any, validate func() error) ([]byte, error) {
	if err := validate(); err != nil {
		return nil, err
	}
	return json.Marshal(value)
}

func decodeStrict(reader io.Reader, limit int64, out any) error {
	if reader == nil {
		return errors.New("JSON reader is required")
	}
	limited := &io.LimitedReader{R: reader, N: limit + 1}
	raw, err := io.ReadAll(limited)
	if err != nil {
		return err
	}
	if int64(len(raw)) > limit {
		return fmt.Errorf("JSON body exceeds %d bytes", limit)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("JSON body contains trailing data")
		}
		return err
	}
	return nil
}
