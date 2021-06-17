package script

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/cloudradar-monitoring/rport/share/query"

	chshare "github.com/cloudradar-monitoring/rport/share"

	errors2 "github.com/cloudradar-monitoring/rport/server/api/errors"
)

var supportedFields = map[string]bool{
	"id":         true,
	"name":       true,
	"created_by": true,
	"created_at": true,
}

type DbProvider interface {
	GetByID(ctx context.Context, id string) (val *Script, found bool, err error)
	List(ctx context.Context, lo *query.ListOptions) ([]Script, error)
	Save(ctx context.Context, s *Script, nowDate time.Time) (string, error)
	Delete(ctx context.Context, id string) error
	io.Closer
}

type Manager struct {
	db     DbProvider
	logger *chshare.Logger
	*Executor
}

func NewManager(db DbProvider, ex *Executor, logger *chshare.Logger) *Manager {
	return &Manager{
		db:       db,
		logger:   logger,
		Executor: ex,
	}
}

func (m *Manager) List(ctx context.Context, re *http.Request) ([]Script, error) {
	listOptions := query.ConvertGetParamsToFilterOptions(re)

	err := query.ValidateListOptions(listOptions, supportedFields)
	if err != nil {
		return nil, err
	}

	return m.db.List(ctx, listOptions)
}

func (m *Manager) GetOne(ctx context.Context, id string) (*Script, bool, error) {
	val, found, err := m.db.GetByID(ctx, id)
	if err != nil {
		return nil, false, err
	}

	if !found {
		return nil, false, nil
	}

	return val, true, nil
}

func (m *Manager) Create(ctx context.Context, valueToStore *InputScript, username string) (*Script, error) {
	err := Validate(valueToStore)
	if err != nil {
		return nil, err
	}

	existingScript, err := m.db.List(ctx, &query.ListOptions{
		Filters: []query.FilterOption{
			{
				Column: "name",
				Values: []string{valueToStore.Name},
			},
		},
	})
	if err != nil {
		return nil, err
	}
	if len(existingScript) > 0 {
		return nil, errors2.APIError{
			Message: fmt.Sprintf("another script with the same name '%s' exists", valueToStore.Name),
			Code:    http.StatusConflict,
		}
	}

	now := time.Now()
	scriptToSave := &Script{
		Name:        valueToStore.Name,
		CreatedBy:   username,
		CreatedAt:   now,
		Interpreter: valueToStore.Interpreter,
		IsSudo:      valueToStore.IsSudo,
		Cwd:         valueToStore.Cwd,
		Script:      valueToStore.Script,
	}
	scriptToSave.ID, err = m.db.Save(ctx, scriptToSave, now)
	if err != nil {
		return nil, err
	}

	return scriptToSave, nil
}

func (m *Manager) Update(ctx context.Context, existingID string, valueToStore *InputScript, username string) (*Script, error) {
	err := Validate(valueToStore)
	if err != nil {
		return nil, err
	}

	_, foundByID, err := m.db.GetByID(ctx, existingID)
	if err != nil {
		return nil, err
	}

	if !foundByID {
		return nil, errors2.APIError{
			Message: "cannot find entry by the provided ID",
			Code:    http.StatusNotFound,
		}
	}

	scriptsWithSameName, err := m.db.List(ctx, &query.ListOptions{
		Filters: []query.FilterOption{
			{
				Column: "name",
				Values: []string{valueToStore.Name},
			},
		},
	})
	if err != nil {
		return nil, err
	}

	if len(scriptsWithSameName) > 0 && scriptsWithSameName[0].ID != existingID {
		return nil, errors2.APIError{
			Message: fmt.Sprintf("another script with the same name '%s' exists", valueToStore.Name),
			Code:    http.StatusConflict,
		}
	}

	now := time.Now()
	scriptToSave := &Script{
		ID:          existingID,
		Name:        valueToStore.Name,
		CreatedBy:   username,
		CreatedAt:   now,
		Interpreter: valueToStore.Interpreter,
		IsSudo:      valueToStore.IsSudo,
		Cwd:         valueToStore.Cwd,
		Script:      valueToStore.Script,
	}
	scriptToSave.ID, err = m.db.Save(ctx, scriptToSave, now)
	if err != nil {
		return nil, err
	}

	return scriptToSave, nil
}

func (m *Manager) Delete(ctx context.Context, id string) error {
	_, found, err := m.db.GetByID(ctx, id)
	if err != nil {
		return errors2.APIError{
			Err:  err,
			Code: http.StatusInternalServerError,
		}
	}

	if !found {
		return errors2.APIError{
			Message: "cannot find this entry by the provided id",
			Code:    http.StatusNotFound,
		}
	}

	err = m.db.Delete(ctx, id)
	if err != nil {
		return errors2.APIError{
			Err:  err,
			Code: http.StatusInternalServerError,
		}
	}

	return nil
}

func (m *Manager) Close() error {
	return m.db.Close()
}
