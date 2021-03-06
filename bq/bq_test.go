package bq_test

import (
	"log"
	"testing"
	"time"

	"cloud.google.com/go/bigquery"

	"github.com/m-lab/etl/bq"
	"github.com/m-lab/etl/etl"
	"github.com/m-lab/etl/fake"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

func foobar(vs bigquery.ValueSaver) {
	_, _, _ = vs.Save()
}

func TestMapSaver(t *testing.T) {
	fns := bq.MapSaver{map[string]bigquery.Value{"filename": "foobar"}}
	foobar(&fns)
}

// Item represents a row item.
type Item struct {
	Name   string
	Count  int
	Foobar int `json:"foobar"`
}

//==================================================================================
// These tests hit the backend, to verify expected behavior of table creation and
// access to partitions.  They deliberately have a leading "x" to prevent running
// them in Travis.  We need to find a better way to control whether they run or
// not.
//==================================================================================
func xTestRealPartitionInsert(t *testing.T) {
	tag := "new"
	items := []interface{}{
		Item{Name: tag + "_x0", Count: 17, Foobar: 44},
		Item{Name: tag + "_x1", Count: 12, Foobar: 44}}

	in, err := bq.NewBQInserter(
		etl.InserterParams{"mlab_sandbox", "test2", "_20160201", 10 * time.Second, 1}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if err = in.InsertRow(Item{Name: tag + "_x0", Count: 17, Foobar: 44}); err != nil {
		t.Error(err)
	}
	if err = in.InsertRows(items); err != nil {
		t.Error(err)
	}

	if in.Accepted() != 3 {
		t.Error("Should have accepted three rows")
	}
	in.Flush()
}

//==================================================================================

func TestBasicInsert(t *testing.T) {
	tag := "new"
	items := []interface{}{
		Item{Name: tag + "_x0", Count: 17, Foobar: 44},
		Item{Name: tag + "_x1", Count: 12, Foobar: 44}}

	in, err := fake.NewFakeInserter(
		etl.InserterParams{"mlab_sandbox", "test2", "", 10 * time.Second, 1})
	if err != nil {
		t.Fatal(err)
	}

	if err = in.InsertRow(Item{Name: tag + "_x0", Count: 17, Foobar: 44}); err != nil {
		t.Error(err)
	}
	if err = in.InsertRows(items); err != nil {
		t.Error(err)
	}

	if in.Accepted() != 3 {
		t.Error("Should have accepted three rows")
	}
	in.Flush()
}

func TestBufferingAndFlushing(t *testing.T) {
	var items []interface{}
	items = append(items, Item{Name: "x1", Count: 17, Foobar: 44})
	items = append(items, Item{Name: "x2", Count: 12, Foobar: 44})

	// Set up an Inserter with a fake Uploader backend for testing.
	// Buffer 3 rows, so that we can test the buffering.
	in, err := fake.NewFakeInserter(
		etl.InserterParams{"mlab_sandbox", "test2", "", 10 * time.Second, 3})
	if err != nil {
		log.Printf("%v\n", err)
		t.Fatal()
	}

	// Try inserting a single row.  Should stay in the buffer.
	if err = in.InsertRow(Item{Name: "x0", Count: 17, Foobar: 44}); err != nil {
		t.Fatal()
	}
	if in.RowsInBuffer() != 1 {
		t.Error("RowsInBuffer = ", in.RowsInBuffer())
	}

	// Insert two more rows, which should trigger a flush.
	if err = in.InsertRows(items); err != nil {
		t.Fatal()
	}
	if in.RowsInBuffer() != 0 {
		t.Error("RowsInBuffer = ", in.RowsInBuffer())
	}
	if in.Accepted() != 3 {
		t.Error("Accepted = ", in.Accepted())
	}

	// Insert two more rows, which should NOT trigger a flush.
	if err = in.InsertRows(items); err != nil {
		t.Fatal()
	}
	if in.RowsInBuffer() != 2 {
		t.Error("RowsInBuffer = ", in.RowsInBuffer())
	}
	if in.Accepted() != 5 {
		t.Error("Accepted = ", in.Accepted())
	}

	// Insert two more rows, which should trigger a flush, and leave one
	// row in the buffer.
	if err = in.InsertRows(items); err != nil {
		t.Fatal()
	}
	if in.RowsInBuffer() != 1 {
		t.Error("RowsInBuffer = ", in.RowsInBuffer())
	}
	if in.Accepted() != 7 {
		t.Error("Accepted = ", in.Accepted())
	}

	// Flush the final row.
	in.Flush()
	if in.RowsInBuffer() != 0 {
		t.Error("RowsInBuffer = ", in.RowsInBuffer())
	}
	if in.Accepted() != 7 {
		t.Error("Count = ", in.Accepted())
	}

}

// Just manual testing for now - need to assert something useful.
func TestHandleInsertErrors(t *testing.T) {
	in, e := bq.NewBQInserter(
		etl.InserterParams{"dataset", "table", "", time.Minute, 5},
		fake.NewFakeUploader())
	if e != nil {
		log.Printf("%v\n", e)
		t.Fatal()
	}

	rie := bigquery.RowInsertionError{InsertID: "1234", RowIndex: 123}
	var bqe bigquery.Error
	bqe.Location = "location"
	bqe.Message = "message"
	bqe.Reason = "invalid"
	// This is a little wierd.  MultiError we receive from insert contain
	// *bigquery.Error.  So that is what we test here.
	rie.Errors = append(rie.Errors, &bqe)

	var pme bigquery.PutMultiError
	pme = append(pme, rie)
	in.(*bq.BQInserter).HandleInsertErrors(pme)

	// TODO - assert something.
}
