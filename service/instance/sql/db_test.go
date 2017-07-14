package sql

import (
	"errors"
	"io/ioutil"
	"testing"

	"github.com/weaveworks/flux/db"
	"github.com/weaveworks/flux/service"
	"github.com/weaveworks/flux/service/instance"
)

func newDB(t *testing.T) *DB {
	f, err := ioutil.TempFile("", "fluxy-testdb")
	if err != nil {
		t.Fatal(err)
	}
	dbsource := "file://" + f.Name()
	if _, err = db.Migrate(dbsource, "../../../db/migrations"); err != nil {
		t.Fatal(err)
	}
	db, err := New("ql", dbsource)
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func TestUpdateOK(t *testing.T) {
	db := newDB(t)

	inst := service.InstanceID("floaty-womble-abc123")
	c := instance.Config{
		Settings: service.InstanceConfig{
			Slack: service.NotifierConfig{
				Username: "test Slack user",
			},
		},
	}
	err := db.UpdateConfig(inst, func(_ instance.Config) (instance.Config, error) {
		return c, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	c1, err := db.GetConfig(inst)
	if err != nil {
		t.Fatal(err)
	}
	if c1.Settings.Slack.Username != c.Settings.Slack.Username {
		t.Errorf("expected config %#v, got %#v", c.Settings, c1.Settings)
	}
}

func TestUpdateRollback(t *testing.T) {
	db := newDB(t)

	inst := service.InstanceID("floaty-womble-abc123")
	c := instance.Config{
		Settings: service.InstanceConfig{
			Slack: service.NotifierConfig{
				Username: "test Slack user",
			},
		},
	}

	err := db.UpdateConfig(inst, func(_ instance.Config) (instance.Config, error) {
		return c, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	err = db.UpdateConfig(inst, func(_ instance.Config) (instance.Config, error) {
		return instance.Config{}, errors.New("arbitrary fail")
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	c1, err := db.GetConfig(inst)
	if err != nil {
		t.Fatal(err)
	}
	if c1.Settings.Slack.Username != c.Settings.Slack.Username {
		t.Errorf("expected config %#v, got %#v", c.Settings, c1.Settings)
	}
}
