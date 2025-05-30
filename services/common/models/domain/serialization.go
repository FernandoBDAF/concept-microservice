package domain

import (
	"encoding/json"
	"encoding/xml"
	"time"
)

// Serializer interface defines methods for model serialization
type Serializer interface {
	ToJSON() ([]byte, error)
	FromJSON([]byte) error
	ToXML() ([]byte, error)
	FromXML([]byte) error
}

// BaseSerializer provides common serialization functionality
type BaseSerializer struct {
	Model interface{}
}

// ToJSON converts the model to JSON
func (s *BaseSerializer) ToJSON() ([]byte, error) {
	return json.Marshal(s.Model)
}

// FromJSON converts JSON to the model
func (s *BaseSerializer) FromJSON(data []byte) error {
	return json.Unmarshal(data, s.Model)
}

// ToXML converts the model to XML
func (s *BaseSerializer) ToXML() ([]byte, error) {
	return xml.Marshal(s.Model)
}

// FromXML converts XML to the model
func (s *BaseSerializer) FromXML(data []byte) error {
	return xml.Unmarshal(data, s.Model)
}

// CustomTime is a custom time type that implements custom JSON and XML marshaling
type CustomTime struct {
	time.Time
}

// MarshalJSON implements custom JSON marshaling for CustomTime
func (t CustomTime) MarshalJSON() ([]byte, error) {
	if t.Time.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(t.Time.Format(time.RFC3339))
}

// UnmarshalJSON implements custom JSON unmarshaling for CustomTime
func (t *CustomTime) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	if s == "null" {
		t.Time = time.Time{}
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return err
	}
	t.Time = parsed
	return nil
}

// MarshalXML implements custom XML marshaling for CustomTime
func (t CustomTime) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if t.Time.IsZero() {
		return e.EncodeElement("", start)
	}
	return e.EncodeElement(t.Time.Format(time.RFC3339), start)
}

// UnmarshalXML implements custom XML unmarshaling for CustomTime
func (t *CustomTime) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var s string
	if err := d.DecodeElement(&s, &start); err != nil {
		return err
	}
	if s == "" {
		t.Time = time.Time{}
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return err
	}
	t.Time = parsed
	return nil
}
