package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/influxdb/influxdb/cmd/influx_tsm/b1"
	"github.com/influxdb/influxdb/cmd/influx_tsm/bz1"
	"github.com/influxdb/influxdb/cmd/influx_tsm/tsdb"
)

type ShardReader interface {
	KeyIterator
	Open() error
	Close() error
}

const (
	backupExt = "bak"
	tsmExt    = "tsm"
)

var description = fmt.Sprintf(`
Convert a database from b1 or bz1 format to tsm1 format.

This tool will make backup any directory before conversion. It
is up to the end-user to delete the backup on the disk. Backups are
named by suffixing the database name with '.%s'. The backups will
be ignored by the system since they are not registered with the cluster.

To restore a backup, delete the tsm version, rename the backup and
restart the node.`, backupExt)

var ds string
var tsmSz uint64

const maxTSMSz = 2 * 1024 * 1024 * 1024

func init() {
	flag.StringVar(&ds, "dbs", "", "Comma-delimited list of databases to convert. Default is to convert all")
	flag.Uint64Var(&tsmSz, "sz", maxTSMSz, "Maximum size of individual TSM files.")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s <data-path> \n", os.Args[0])
		fmt.Fprintf(os.Stderr, "%s\n\n", description)
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\n")
	}
}

func main() {
	flag.Parse()

	if len(flag.Args()) < 1 {
		fmt.Fprintf(os.Stderr, "no data directory specified\n")
		os.Exit(1)
	}
	dataPath := flag.Args()[0]

	if tsmSz > maxTSMSz {
		fmt.Fprintf(os.Stderr, "maximum TSM file size is %d\n", maxTSMSz)
		os.Exit(1)
	}

	// Check if specific directories were requested.
	reqDs := strings.Split(ds, ",")
	if len(reqDs) == 1 && reqDs[0] == "" {
		reqDs = nil
	}

	// Determine the list of databases
	dbs, err := ioutil.ReadDir(dataPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to access data directory at %s: %s\n", dataPath, err.Error())
		os.Exit(1)
	}
	fmt.Println() // Cleanly separate output from start of program.

	// Get the list of shards for conversion.
	var shards []*tsdb.ShardInfo
	for _, db := range dbs {
		if strings.HasSuffix(db.Name(), backupExt) {
			fmt.Printf("Skipping %s as it looks like a backup.\n", db.Name())
			continue
		}

		d := tsdb.NewDatabase(filepath.Join(dataPath, db.Name()))
		shs, err := d.Shards()
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to access shards for database %s: %s\n", d.Name(), err.Error())
			os.Exit(1)
		}
		shards = append(shards, shs...)
	}
	sort.Sort(tsdb.ShardInfos(shards))
	shards = tsdb.ShardInfos(shards).FilterFormat(tsdb.TSM1).FilterDatabases(reqDs)

	// Anything to convert?
	if len(shards) == 0 {
		fmt.Println("Nothing to do.")
		os.Exit(0)
	}

	// Display list of convertible shards.
	fmt.Println()
	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 0, 8, 1, '\t', 0)
	fmt.Fprintln(w, "Database\tRetention\tPath\tEngine\tSize")
	for _, si := range shards {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\n", si.Database, si.RetentionPolicy, si.FullPath(dataPath), si.FormatAsString(), si.Size)
	}
	w.Flush()

	// Get confirmation from user.
	fmt.Printf("\nThese shards will be converted. Proceed? y/N: ")
	liner := bufio.NewReader(os.Stdin)
	yn, err := liner.ReadString('\n')
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read response: %s", err.Error())
		os.Exit(1)
	}
	yn = strings.TrimRight(strings.ToLower(yn), "\n")
	if yn != "y" {
		fmt.Println("Conversion aborted.")
		os.Exit(1)
	}
	fmt.Println("Conversion starting....")

	// Backup each directory.
	for _, db := range tsdb.ShardInfos(shards).Databases() {
		dest := filepath.Join(dataPath, db+"."+backupExt)
		src := filepath.Join(dataPath, db)

		if _, err := os.Stat(dest); !os.IsNotExist(err) {
			fmt.Printf("Backup of database %s already exists\n", db)
			os.Exit(1)
		}

		err = copyDir(dest, src)
		if err != nil {
			fmt.Printf("Backup of database %s failed: %s\n", db, err.Error())
			os.Exit(1)
		}
		fmt.Printf("Database %s backed up to %s\n", db, dest)
	}

	// Convert each shard.
	for _, si := range shards {
		src := si.FullPath(dataPath)
		dst := fmt.Sprintf("%s.%s", src, tsmExt)

		var reader ShardReader
		switch si.Format {
		case tsdb.BZ1:
			reader = bz1.NewReader(src)
		case tsdb.B1:
			reader = b1.NewReader(src)
		default:
			fmt.Printf("Unsupported shard format: %s\n", si.FormatAsString())
			os.Exit(1)
		}

		// Open the shard, and create a converter.
		if err := reader.Open(); err != nil {
			fmt.Printf("Failed to open %s for conversion: %s\n", src, err.Error())
			os.Exit(1)
		}
		converter := NewConverter(dst, uint32(tsmSz))

		// Perform the conversion.
		start := time.Now()
		if err := converter.Process(reader); err != nil {
			fmt.Printf("Conversion of %s failed: %s\n", src, err.Error())
			os.Exit(1)
		}

		// Delete source shard, and rename new tsm1 shard.
		if err := reader.Close(); err != nil {
			fmt.Printf("Conversion of %s failed due to close: %s\n", src, err.Error())
			os.Exit(1)
		}

		if err := os.RemoveAll(si.FullPath(dataPath)); err != nil {
			fmt.Printf("Deletion of %s failed: %s\n", src, err.Error())
			os.Exit(1)
		}
		if err := os.Rename(dst, src); err != nil {
			fmt.Printf("Rename of %s to %s failed: %s", dst, src, err.Error())
			os.Exit(1)
		}

		// Success!
		fmt.Printf("Conversion of %s successful (%s)\n", src, time.Now().Sub(start))
	}
}

// copyDir copies the directory at src to dest. If dest does not exist it
// will be created. It is up to the caller to ensure the paths don't overlap.
func copyDir(dest, src string) error {
	copyFile := func(path string, info os.FileInfo, err error) error {
		// Strip the src from the path and replace with dest.
		toPath := strings.Replace(path, src, dest, 1)

		// Copy it.
		if info.IsDir() {
			if err := os.MkdirAll(toPath, info.Mode()); err != nil {
				return err
			}
		} else {
			err := func() error {
				in, err := os.Open(path)
				if err != nil {
					return err
				}
				defer in.Close()

				out, err := os.OpenFile(toPath, os.O_CREATE|os.O_WRONLY, info.Mode())
				if err != nil {
					return err
				}
				defer out.Close()

				_, err = io.Copy(out, in)
				return err
			}()
			if err != nil {
				return err
			}
		}
		return nil
	}

	return filepath.Walk(src, copyFile)
}
