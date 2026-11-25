package check

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"regexp"
	"strings"
)

var (
	notNull     = regexp.MustCompile(`NOT NULL constraint failed: [^.\s]+\.(\w+)`)
	importTable = regexp.MustCompile("(?i)(?:\\bold\\s*\\.\\s*|\\binto\\s+(?:main\\s*\\.\\s*)?)[\"`\\[]?(\\w+)")
)

// dataCopy does to the app's database what llocal's copy of the old data does: into emptied
// tables, a row of each old table whose columns the survey read goes in by name, with only
// the columns both have. Tables db/import.sql fills or reads are left to it. The row's values
// suit the new columns' types, so what stops it is what stops the real copy: a column the new
// table requires and the old one lacks, or no column in common.
func (c *checker) dataCopy(ctx context.Context, file string, tables []Table) error {
	// Foreign keys are off, as in the real copy: a row's references are other rows' business.
	db, err := sql.Open("sqlite", file+"?_pragma=foreign_keys(0)")
	if err != nil {
		return err
	}
	defer db.Close()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	script, _ := fs.ReadFile(c.root.FS(), "db/import.sql")
	done := map[string]bool{} // first, the tables db/import.sql reads from the old data or fills
	for _, m := range importTable.FindAllSubmatch(script, -1) {
		done[strings.ToLower(string(m[1]))] = true
	}
	var failed []string
	for _, t := range tables {
		key := strings.ToLower(t.Name)
		if len(t.Columns) == 0 || done[key] {
			continue
		}
		done[key] = true
		var name string
		if err := tx.QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type = 'table' AND name = ? COLLATE NOCASE`, t.Name).Scan(&name); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM `+ident(name)); err != nil {
			return err
		}
		rows, err := tx.QueryContext(ctx, `SELECT lower(name), type FROM pragma_table_info(?)`, name)
		if err != nil {
			return err
		}
		types := map[string]string{}
		for rows.Next() {
			var col, typ string
			if err := rows.Scan(&col, &typ); err != nil {
				rows.Close()
				return err
			}
			types[col] = typ
		}
		if err := rows.Close(); err != nil {
			return err
		}
		var cols, marks []string
		var row []any
		for _, col := range t.Columns {
			if typ, ok := types[strings.ToLower(col)]; ok {
				cols, marks, row = append(cols, ident(col)), append(marks, "?"), append(row, sample(typ))
			}
		}
		if len(cols) == 0 {
			failed = append(failed, fmt.Sprintf("llocal's copy of the old data can't fill table %q: it has none of the old table's columns (%s), and rows are copied by column name", name, strings.Join(t.Columns, ", ")))
			continue
		}
		_, err = tx.ExecContext(ctx, fmt.Sprintf(`INSERT INTO %s (%s) VALUES (%s)`, ident(name), strings.Join(cols, ", "), strings.Join(marks, ", ")), row...)
		switch m := notNull.FindStringSubmatch(fmt.Sprint(err)); {
		case err == nil || strings.Contains(err.Error(), "CHECK constraint failed"):
			// A sample row can't know what a CHECK wants; the old rows met it.
		case m != nil:
			failed = append(failed, fmt.Sprintf("llocal's copy of the old data can't fill table %q: its column %s is required and the old %s table has no %s; let it be empty or give it a default, so the old rows come across, or fill the table in db/import.sql", name, m[1], t.Name, m[1]))
		default:
			failed = append(failed, fmt.Sprintf("llocal's copy of the old data can't fill table %q: %v", name, err))
		}
	}
	return joined(failed)
}

// sample is a value a column of SQLite type typ takes.
func sample(typ string) any {
	t := strings.ToUpper(typ)
	for _, number := range []string{"INT", "REAL", "FLOA", "DOUB", "NUM", "DEC", "BOOL"} {
		if strings.Contains(t, number) {
			return 1
		}
	}
	if strings.Contains(t, "BLOB") {
		return []byte("1")
	}
	return "1"
}

func ident(name string) string { return `"` + strings.ReplaceAll(name, `"`, `""`) + `"` }
