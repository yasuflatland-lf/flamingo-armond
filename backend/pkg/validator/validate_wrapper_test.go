package validator_test

import (
	"backend/graph/db"
	"backend/pkg/validator"
	"backend/testutils"
	"testing"
	"time"
)

func TestValidation(t *testing.T) {
	validateWrapper := validator.NewValidateWrapper()

	tests := []struct {
		name  string
		field string
		value interface{}
		tag   string
		valid bool
	}{
		// Valid cases
		{"Valid User Name", "fl_name", "ValidName", "required,alpha", true},
		{"Valid User Name2", "fl_name", "Tanaka Hideyuki", "required,fl_name", true},
		{"Valid User Name3", "fl_name", "Raphaël Maël", "required,fl_name", true},

		// Invalid cases
		{"Invalid User Name", "fl_name", "Invalid Name!", "required,fl_name", false},
		{"Empty User Name", "fl_name", "", "required,fl_name", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateWrapper.Validator().Var(tt.value, tt.tag)
			if (err == nil) != tt.valid {
				t.Errorf("expected valid: %v, got error: %v", tt.valid, err)
			}
		})
	}
}

func TestDateTimeValidation(t *testing.T) {
	validateWrapper := validator.NewValidateWrapper()
	invalidTime, _ := time.Parse(time.Now().UTC().Format(time.DateTime), "0000-00-00")
	tests := []struct {
		name  string
		value time.Time
		tag   string
		valid bool
	}{
		// Valid cases
		{"Valid DateTime", time.Now().UTC(), "fl_datetime", true},

		// Invalid cases
		{"Invalid DateTime", invalidTime, "fl_datetime", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateWrapper.Validator().Var(tt.value, tt.tag)
			if (err == nil) != tt.valid {
				t.Errorf("expected valid: %v, got error: %v", tt.valid, err)
			}
		})
	}
}

func TestModelValidation(t *testing.T) {
	contributor := validator.NewValidateWrapper()

	tests := []struct {
		name  string
		model interface{}
		valid bool
	}{
		// Valid cases
		{
			"Valid User",
			&db.User{
				ID:       1,
				Name:     "ValidName",
				Email:    testutils.GetRandomEmail(8),
				GoogleID: testutils.GenerateUUIDv7(),
				Created:  time.Now().UTC(),
				Updated:  time.Now().UTC(),
			},
			true,
		},
		{
			"Valid Card",
			&db.Card{
				ID:           1,
				Front:        "Front Text",
				Back:         "Back Text",
				ReviewDate:   time.Now().UTC(),
				IntervalDays: 1,
				Created:      time.Now().UTC(),
				Updated:      time.Now().UTC(),
				CardGroupID:  1,
			},
			true,
		},
		{
			"Valid Cardgroup",
			&db.Cardgroup{
				ID:      1,
				Name:    "Group Name",
				Created: time.Now().UTC(),
				Updated: time.Now().UTC(),
			},
			true,
		},
		{
			"Valid Role",
			&db.Role{
				ID:      1,
				Name:    "Role Name",
				Created: time.Now().UTC(),
				Updated: time.Now().UTC(),
			},
			true,
		},

		// Invalid cases
		{
			"Invalid User Name",
			&db.User{
				ID:       1,
				Name:     "Invalid Name!",
				Email:    testutils.GetRandomEmail(8),
				GoogleID: testutils.GenerateUUIDv7(),
				Created:  time.Now().UTC(),
				Updated:  time.Now().UTC(),
			},
			false,
		},
		{
			"Invalid Card Review Date",
			&db.Card{
				ID:           1,
				Front:        "Front Text",
				Back:         "Back Text",
				ReviewDate:   time.Time{},
				IntervalDays: 1,
				Created:      time.Now().UTC(),
				Updated:      time.Now().UTC(),
				CardGroupID:  1,
			},
			false,
		},
		{
			"Invalid Cardgroup Name",
			&db.Cardgroup{
				ID:      1,
				Name:    "",
				Created: time.Now().UTC(),
				Updated: time.Now().UTC(),
			},
			false,
		},
		{
			"Invalid Role Name",
			&db.Role{
				ID:      1,
				Name:    "Invalid Role Name!",
				Created: time.Now().UTC(),
				Updated: time.Now().UTC(),
			},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := contributor.Validator().Struct(tt.model)
			if (err == nil) != tt.valid {
				t.Errorf("expected valid: %v, got error: %v", tt.valid, err)
			}
		})
	}
}
