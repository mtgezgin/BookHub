package ratings

import (
	"reflect"
	"testing"
	"time"

	"github.com/mtgezgin/bookhub/backend/internal/db"
)

// fillNonZero, struct'ın her alanına sıfır OLMAYAN bir değer yazar (pointer'lar dahil).
func fillNonZero(t *testing.T, v reflect.Value) {
	t.Helper()
	for i := range v.NumField() {
		f := v.Field(i)
		switch f.Kind() {
		case reflect.String:
			f.SetString("x-" + v.Type().Field(i).Name)
		case reflect.Int16, reflect.Int32, reflect.Int64:
			f.SetInt(int64(i + 1))
		case reflect.Float64:
			f.SetFloat(float64(i) + 0.5)
		case reflect.Pointer:
			p := reflect.New(f.Type().Elem())
			switch p.Elem().Kind() {
			case reflect.String:
				p.Elem().SetString("p")
			case reflect.Int32, reflect.Int64:
				p.Elem().SetInt(7)
			case reflect.Float64:
				p.Elem().SetFloat(7.5)
			case reflect.Struct: // *time.Time
				p.Elem().Set(reflect.ValueOf(time.Unix(1700000000, 0)))
			}
			f.Set(p)
		case reflect.Struct: // time.Time
			f.Set(reflect.ValueOf(time.Unix(1700000000, 0)))
		default:
			t.Fatalf("testin tanımadığı alan tipi: %s %s", v.Type().Field(i).Name, f.Kind())
		}
	}
}

// TestBookFromTopRatedRowCopiesEveryField (review bulgusu R5): db.Book'un HER alanı
// top-rated satırından taşınmalı. books'a yeni kolon eklenip kopya unutulursa bu test,
// alanın adını söyleyerek kırılır.
func TestBookFromTopRatedRowCopiesEveryField(t *testing.T) {
	var row db.ListTopRatedRow
	fillNonZero(t, reflect.ValueOf(&row).Elem())

	got := reflect.ValueOf(bookFromTopRatedRow(row))
	src := reflect.ValueOf(row)
	for i := range got.NumField() {
		name := got.Type().Field(i).Name
		want := src.FieldByName(name)
		if !want.IsValid() {
			t.Errorf("db.Book.%s top-rated satırında yok — ListTopRated sorgusu b.* seçmeli", name)
			continue
		}
		if !reflect.DeepEqual(got.Field(i).Interface(), want.Interface()) {
			t.Errorf("db.Book.%s kopyalanmadı (bookFromTopRatedRow'a ekle)", name)
		}
	}
}
