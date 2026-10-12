package taskimage

import (
	"reflect"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestReplaceAndKeysForTasks(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:task_images_store?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		"CREATE TABLE tasks (id integer primary key, image_key text)",
		"CREATE TABLE task_images (id integer primary key, task_id integer, image_key text unique, position integer, created_at datetime)",
		"INSERT INTO tasks (id) VALUES (1), (2)",
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}

	if err := Replace(db, 1, []string{"first", "second"}); err != nil {
		t.Fatal(err)
	}
	if err := Replace(db, 2, []string{"third"}); err != nil {
		t.Fatal(err)
	}
	got, err := KeysForTasks(db, []uint{1, 2})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got[1], []string{"first", "second"}) || !reflect.DeepEqual(got[2], []string{"third"}) {
		t.Fatalf("unexpected image keys: %#v", got)
	}

	if err := Replace(db, 1, []string{"second"}); err != nil {
		t.Fatal(err)
	}
	var first *string
	if err := db.Raw("SELECT image_key FROM tasks WHERE id = 1").Scan(&first).Error; err != nil {
		t.Fatal(err)
	}
	if first == nil || *first != "second" {
		t.Fatalf("legacy first image = %#v", first)
	}
	if err := Replace(db, 1, nil); err != nil {
		t.Fatal(err)
	}
	if err := db.Raw("SELECT image_key FROM tasks WHERE id = 1").Scan(&first).Error; err != nil {
		t.Fatal(err)
	}
	if first != nil {
		t.Fatalf("expected legacy image key to be cleared, got %q", *first)
	}
}

func TestReplaceRejectsTooManyImages(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:task_images_limit?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	keys := []string{"1", "2", "3", "4", "5", "6"}
	if err := Replace(db, 1, keys); err == nil {
		t.Fatal("expected too many images to be rejected")
	}
}
