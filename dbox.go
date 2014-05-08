/*
** Copyright (c) 2014 Arnaud Ysmal.  All Rights Reserved.
**
** Redistribution and use in source and binary forms, with or without
** modification, are permitted provided that the following conditions
** are met:
** 1. Redistributions of source code must retain the above copyright
**    notice, this list of conditions and the following disclaimer.
** 2. Redistributions in binary form must reproduce the above copyright
**    notice, this list of conditions and the following disclaimer in the
**    documentation and/or other materials provided with the distribution.
**
** THIS SOFTWARE IS PROVIDED BY THE AUTHOR ``AS IS'' AND ANY EXPRESS
** OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
** WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
** DISCLAIMED. IN NO EVENT SHALL THE AUTHOR OR CONTRIBUTORS BE LIABLE
** FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
** DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
** SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION)
** HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT
** LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY
** OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF
** SUCH DAMAGE.
 */

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/stacktic/dropbox"
)

const appKey = ""
const appSecret = ""
const configFilename = ".dbox"

// ConfigFile represents the structure of the configuration file.
type ConfigFile struct {
	Token   string `json:"token"`
	Key     []byte `json:"key"`
	changed bool   `json:"-"`
}

// Read reads the named configuration file.
func (cf *ConfigFile) Read(fname string) error {
	var err error
	var file string
	var buf []byte

	file = filepath.Join(os.Getenv("HOME"), fname)
	if buf, err = ioutil.ReadFile(file); err == nil {
		err = json.Unmarshal(buf, cf)
	}
	return err
}

// Write writes the named configuration file.
func (cf *ConfigFile) Write(fname string) error {
	var err error
	var file string
	var buf []byte

	file = filepath.Join(os.Getenv("HOME"), fname)
	if buf, err = json.MarshalIndent(cf, "", " "); err == nil {
		err = ioutil.WriteFile(file, buf, 0600)
	}
	return err
}

type cmdHandler func(*ConfigFile, *dropbox.Dropbox, []string) error

func printEntry(entry *dropbox.Entry, prefixlen int) {
	var buffer bytes.Buffer

	if prefixlen != 0 {
		prefixlen--
		for prefixlen < len(entry.Path) && entry.Path[prefixlen] != '/' {
			prefixlen++
		}
		prefixlen++
	}
	buffer.WriteString(entry.Path[prefixlen:])
	if entry.IsDir {
		buffer.WriteByte('/')
	}
	if entry.IsDeleted {
		buffer.WriteString(" [deleted]")
	}
	fmt.Println(buffer.String())
}

func printEntryLong(entry *dropbox.Entry, prefixlen int) {
	var buffer bytes.Buffer
	var entryTime time.Time

	if prefixlen != 0 && entry.Path[prefixlen] == '/' {
		prefixlen++
	}
	buffer.WriteString(entry.Path[prefixlen:])
	if entry.IsDir && entry.Path != "/" {
		buffer.WriteByte('/')
	}
	buffer.WriteString(fmt.Sprintf("\t%s\t", entry.Size))

	entryTime = time.Time(entry.Modified)
	if !entryTime.IsZero() {
		buffer.WriteString(fmt.Sprintf("%s\t", entryTime.Format(dropbox.DateFormat)))
	} else {
		buffer.WriteString(fmt.Sprintf("%*s\t", len(dropbox.DateFormat), ""))
	}

	buffer.WriteString(fmt.Sprintf("%s", entry.Revision))
	if entry.IsDeleted {
		buffer.WriteString("\t[deleted]")
	}
	fmt.Println(buffer.String())
}

func printEntriesLong(entries []dropbox.Entry, prefixlen int) {
	var psize, ssize, rsize int
	var i int
	var buffer bytes.Buffer
	var entryTime time.Time

	if prefixlen != 0 && entries[0].Path[prefixlen] == '/' {
		prefixlen++
	}
	for i = range entries {
		if len(entries[i].Path) > psize {
			psize = len(entries[i].Path)
		}
		if len(entries[i].Size) > ssize {
			ssize = len(entries[i].Size)
		}
		if len(entries[i].Revision) > rsize {
			rsize = len(entries[i].Revision)
		}
	}
	psize = psize + 1 - prefixlen
	for i = range entries {
		name := entries[i].Path[prefixlen:]
		if entries[i].IsDir && name != "/" {
			name += "/"
		}
		buffer.WriteString(fmt.Sprintf("%-*s\t%-*s\t", psize, name, ssize, entries[i].Size))

		entryTime = time.Time(entries[i].Modified)
		if !entryTime.IsZero() {
			buffer.WriteString(fmt.Sprintf("%s\t", entryTime.Format(dropbox.DateFormat)))
		} else {
			buffer.WriteString(fmt.Sprintf("%*s\t", len(dropbox.DateFormat), ""))
		}

		buffer.WriteString(fmt.Sprintf("%-*s\t", rsize, entries[i].Revision))
		if entries[i].IsDeleted {
			buffer.WriteString("\t[deleted]")
		}
		fmt.Println(buffer.String())
		buffer.Reset()
	}
}

func doChunkedPut(config *ConfigFile, db *dropbox.Dropbox, params []string) error {
	var cl *flag.FlagSet
	var crypt, keep bool
	var files []string
	var err error
	var rev string
	var fd *os.File
	var chunksize int
	var fsize int
	var reader io.ReadCloser
	var fi os.FileInfo

	cl = flag.NewFlagSet("cput", flag.ExitOnError)
	cl.BoolVar(&crypt, "aes", false, "Crypt file with AES before sending them.")
	cl.IntVar(&chunksize, "c", dropbox.DefaultChunkSize, "Size of the chunk")
	cl.BoolVar(&keep, "k", false, "Do not overwrite if exists.")
	cl.StringVar(&rev, "r", "", "Revision of the file overwritten.")
	cl.Parse(params)

	files = cl.Args()

	if fd, err = os.Open(files[0]); err != nil {
		return err
	}
	defer fd.Close()

	if len(files) != 2 {
		return fmt.Errorf("exactly two parameters needed for put (source and destination)")
	}

	if crypt {
		if len(config.Key) == 0 {
			config.Key, _ = dropbox.GenerateKey(32)
			config.changed = true
		}
		if fi, err = fd.Stat(); err != nil {
			return err
		}
		fsize = int(fi.Size())
		if reader, _, err = dropbox.NewAESCrypterReader(config.Key, fd, fsize); err != nil {
			return err
		}
	} else {
		reader = fd
	}
	if _, err = db.UploadByChunk(reader, chunksize, files[1], !keep, rev); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", files[0], err)
	} else {
		fmt.Printf("%s\n", files[1])
	}
	return nil
}

func doCopy(config *ConfigFile, db *dropbox.Dropbox, params []string) error {
	var cl *flag.FlagSet
	var err error
	var copyref bool

	cl = flag.NewFlagSet("copy", flag.ExitOnError)
	cl.BoolVar(&copyref, "r", false, "From is a reference obtained by copy_ref")
	cl.Parse(params)
	params = cl.Args()
	if len(params) != 2 {
		return fmt.Errorf("exactly two parameters needed for move (from path and to path)")
	}

	_, err = db.Copy(params[0], params[1], copyref)
	return err
}

func doCopyRef(config *ConfigFile, db *dropbox.Dropbox, params []string) error {
	var err error
	var file string
	var ref *dropbox.CopyRef

	for _, file = range params {
		if ref, err = db.CopyRef(file); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %s\n", file, err)
			continue
		}
		fmt.Printf("%s: ref: %s expires on %s\n", file, ref.CopyRef, ref.Expires)
	}
	return nil
}

func doDelta(config *ConfigFile, db *dropbox.Dropbox, params []string) error {
	var cl *flag.FlagSet
	var err error
	var cursor, prefix string
	var dp *dropbox.DeltaPage

	cl = flag.NewFlagSet("delta", flag.ExitOnError)
	cl.StringVar(&cursor, "c", "", "Cursor of the current stat")
	cl.StringVar(&prefix, "p", "", "Path prefix for deltas")
	cl.Parse(params)

	dp, err = db.Delta(cursor, prefix)
	if err != nil {
		return err
	}
	for _, entry := range dp.Entries {
		if entry.Entry == nil {
			fmt.Printf("%s: deleted\n", entry.Path)
		} else {
			fmt.Printf("%s: %#v\n", entry.Path, *entry.Entry)
		}
	}
	return err
}

func doGet(config *ConfigFile, db *dropbox.Dropbox, params []string) error {
	var cl *flag.FlagSet
	var cont, crypt bool
	var files []string
	var err error
	var rev string

	cl = flag.NewFlagSet("get", flag.ExitOnError)
	cl.BoolVar(&crypt, "aes", false, "Crypt file with AES before sending them.")
	cl.BoolVar(&cont, "c", false, "Resume download.")
	cl.StringVar(&rev, "r", "", "Download the file at the specified revision.")
	cl.Parse(params)
	files = cl.Args()

	if cont && crypt {
		return fmt.Errorf("-aes and -c are mutually exclusive")
	}
	if len(files) != 2 {
		return fmt.Errorf("exactly two parameters needed for get (source and destination)")
	}

	if crypt {
		if len(config.Key) == 0 {
			config.Key, _ = dropbox.GenerateKey(32)
			config.changed = true
		}
		err = db.DownloadToFileAES(config.Key, files[0], files[1], rev)
	} else {
		if cont {
			err = db.DownloadToFileResume(files[0], files[1], rev)
		} else {
			err = db.DownloadToFile(files[0], files[1], rev)
		}
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", files[0], err)
	} else {
		fmt.Printf("%s\n", files[1])
	}
	return nil
}

func doList(config *ConfigFile, db *dropbox.Dropbox, params []string) error {
	var cl *flag.FlagSet
	var all, long, nochild bool
	var files []string
	var entry *dropbox.Entry
	var subentry dropbox.Entry
	var err error
	var display func(*dropbox.Entry, int)

	cl = flag.NewFlagSet("list", flag.ExitOnError)
	cl.BoolVar(&all, "a", false, "Show deleted entries.")
	cl.BoolVar(&nochild, "d", false, "Do not show children for a directory.")
	cl.BoolVar(&long, "l", false, "Display long format.")
	cl.Parse(params)

	if long {
		display = printEntryLong
	} else {
		display = printEntry
	}

	files = cl.Args()
	if len(files) == 0 {
		files = []string{"/"}
	}
	for i, file := range files {
		file = strings.TrimRight(file, "/")
		if entry, err = db.Metadata(file, !nochild, all, "", "", 0); err != nil {
			fmt.Println(err)
			continue
		}
		if entry.IsDir {
			display(entry, 0)
			if len(entry.Contents) == 0 {
				continue
			}
			fmt.Println("")
			if long {
				printEntriesLong(entry.Contents, len(file))
			} else {
				for _, subentry = range entry.Contents {
					printEntry(&subentry, len(file))
				}
			}
		} else {
			display(entry, 0)
		}
		if i < len(files)-1 {
			fmt.Println("")
		}
	}
	return nil
}

func doLongPollDelta(config *ConfigFile, db *dropbox.Dropbox, params []string) error {
	var cl *flag.FlagSet
	var err error
	var timeout int
	var dp *dropbox.DeltaPoll

	cl = flag.NewFlagSet("longpoll_delta", flag.ExitOnError)
	cl.IntVar(&timeout, "t", 30, "Timeout")
	cl.Parse(params)
	params = cl.Args()
	if len(params) != 1 {
		return fmt.Errorf("exactly one parameter needed for ldelta (cursor)")
	}
	if dp, err = db.LongPollDelta(params[0], timeout); err != nil {
		return err
	}
	if dp.Changes {
		fmt.Printf("You may now call delta with cursor %s\n", params[0])
	} else {
		fmt.Printf("No changes")
	}
	return nil
}

func doMedia(config *ConfigFile, db *dropbox.Dropbox, params []string) error {
	var link *dropbox.Link
	var err error

	for _, file := range params {
		if link, err = db.Media(file); err != nil {
			fmt.Printf("%s: %s\n", file, err)
			continue
		}
		fmt.Printf("%s is now available using %s, this link expires on %s\n",
			file, link.URL, link.Expires)
	}
	return nil
}

func doMkdir(config *ConfigFile, db *dropbox.Dropbox, params []string) error {
	var err error

	for _, file := range params {
		if _, err = db.CreateFolder(file); err != nil {
			fmt.Printf("%s: %s\n", file, err)
			continue
		}
	}
	return nil
}

func doMove(config *ConfigFile, db *dropbox.Dropbox, params []string) error {
	var err error

	if len(params) != 2 {
		return fmt.Errorf("exactly two parameters needed for move (from path and to path)")
	}

	_, err = db.Move(params[0], params[1])
	return err
}

func doPut(config *ConfigFile, db *dropbox.Dropbox, params []string) error {
	var cl *flag.FlagSet
	var crypt, keep bool
	var files []string
	var err error
	var rev string

	cl = flag.NewFlagSet("put", flag.ExitOnError)
	cl.BoolVar(&crypt, "aes", false, "Crypt file with AES before sending them.")
	cl.BoolVar(&keep, "k", false, "Do not overwrite if exists.")
	cl.StringVar(&rev, "r", "", "Revision of the file overwritten.")
	cl.Parse(params)
	files = cl.Args()

	if len(files) != 2 {
		return fmt.Errorf("exactly two parameters needed for put (source and destination)")
	}

	if crypt && len(config.Key) == 0 {
		config.Key, _ = dropbox.GenerateKey(32)
		config.changed = true
	}

	if crypt {
		_, err = db.UploadFileAES(config.Key, files[0], files[1], !keep, rev)
	} else {
		_, err = db.UploadFile(files[0], files[1], !keep, rev)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", files[0], err)
	} else {
		fmt.Printf("%s\n", files[1])
	}
	return nil
}

func doRestore(config *ConfigFile, db *dropbox.Dropbox, params []string) error {
	var err error

	if len(params) != 2 {
		return fmt.Errorf("exactly two parameters needed for restore (path and revision)")
	}

	_, err = db.Restore(params[0], params[1])
	return err
}

func doRevisions(config *ConfigFile, db *dropbox.Dropbox, params []string) error {
	var cl *flag.FlagSet
	var entries []dropbox.Entry
	var entry dropbox.Entry
	var err error
	var lim int

	cl = flag.NewFlagSet("revisions", flag.ExitOnError)
	cl.IntVar(&lim, "l", 10, "Maximum number of revisions.")
	cl.Parse(params)

	for _, file := range params {
		if entries, err = db.Revisions(file, lim); err != nil {
			fmt.Printf("%s: %s\n", file, err)
			continue
		}
		for _, entry = range entries {
			if !entry.IsDeleted || entry.Bytes != 0 {
				printEntryLong(&entry, 0)
			}
		}
	}
	return nil
}

func doRm(config *ConfigFile, db *dropbox.Dropbox, params []string) error {
	var err error

	for _, file := range params {
		if _, err = db.Delete(file); err != nil {
			fmt.Printf("%s: %s\n", file, err)
			continue
		}
	}
	return nil
}

func doSearch(config *ConfigFile, db *dropbox.Dropbox, params []string) error {
	var cl *flag.FlagSet
	var entries []dropbox.Entry
	var entry dropbox.Entry
	var err error
	var all, long bool
	var nb int

	cl = flag.NewFlagSet("search", flag.ExitOnError)
	cl.BoolVar(&all, "a", false, "Show deleted entries.")
	cl.BoolVar(&long, "l", false, "Display long format.")
	cl.IntVar(&nb, "m", 0, "Maximum number of entry.")
	cl.Parse(params)
	params = cl.Args()

	if len(params) != 2 {
		return fmt.Errorf("exactly two parameters needed for search (path and query)")
	}

	if entries, err = db.Search(strings.TrimRight(params[0], "/"), params[1], nb, all); err != nil {
		return err
	}
	fmt.Printf("%s:\n", params[0])
	if long {
		printEntriesLong(entries, len(params[0]))
	} else {
		for _, entry = range entries {
			printEntry(&entry, len(params[0]))
		}
	}
	return nil
}

func doShare(config *ConfigFile, db *dropbox.Dropbox, params []string) error {
	var cl *flag.FlagSet
	var link *dropbox.Link
	var err error
	var orig bool

	cl = flag.NewFlagSet("share", flag.ExitOnError)
	cl.BoolVar(&orig, "o", true, "Get the original URL")
	cl.Parse(params)
	params = cl.Args()

	for _, file := range params {
		if link, err = db.Shares(file, !orig); err != nil {
			fmt.Printf("%s: %s\n", file, err)
			continue
		}
		fmt.Printf("%s is now available using %s, this link expires on %s\n",
			file, link.URL, link.Expires)
	}
	return nil
}

func doThumbnails(config *ConfigFile, db *dropbox.Dropbox, params []string) error {
	var cl *flag.FlagSet
	var err error
	var size, format string

	cl = flag.NewFlagSet("thumbnails", flag.ExitOnError)
	cl.StringVar(&size, "s", "s", "Size of the thumbnails (xs, s, m, l or xl)")
	cl.StringVar(&format, "f", "png", "Format of the thumbnails (jpeg or png)")
	cl.Parse(params)
	params = cl.Args()

	if len(params) != 2 {
		return fmt.Errorf("exactly two parameters needed for thumbnails (source and destination)")
	}

	_, err = db.ThumbnailsToFile(params[0], params[1], format, size)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", params[0], err)
	} else {
		fmt.Printf("%s\n", params[1])
	}
	return nil
}

func doHelp(config *ConfigFile, db *dropbox.Dropbox, params []string) error {
	keys := make([]string, 0, len(commands))
	for k := range commands {
		if k != "help" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	fmt.Printf("Command list:\n")
	for _, k := range keys {
		fmt.Printf("%10s: %s\n", k, commands[k].desc)
		fmt.Printf("            Usage: %s %s\n", k, commands[k].usage)
	}
	fmt.Printf("%10s: %s\n", "help", commands["help"].desc)
	return nil
}

type command struct {
	desc  string
	usage string
	Func  cmdHandler
}

var commands = map[string]command{
	"copy":       command{"Copy file or directory.", "[-r] from_file to_file", doCopy},
	"copyref":    command{"Get a copy reference of a file.", "file [files...]", doCopyRef},
	"cput":       command{"Upload a file.", "[-aes] [-c chunksize] [-k] [-r rev] file destination", doChunkedPut},
	"delta":      command{"Get modifications.", "[-c cursor] [-p path_prefix]", doDelta},
	"delete":     command{"Remove file or directory (Warning this remove is recursive).", "file [files...]", doRm},
	"get":        command{"Download a file.", "[-aes] [-c] [-r rev] file destination", doGet},
	"list":       command{"List files from directories.", "[-a] [-d] [-l] [files...]", doList},
	"ldelta":     command{"Get modifications with timeout.", "[-t timeout] cursor", doLongPollDelta},
	"media":      command{"Shares files with direct access.", "file [files...]", doMedia},
	"mkdir":      command{"Create directories.", "directory [directories...]", doMkdir},
	"move":       command{"Move file or directory.", "from_file to_file", doMove},
	"put":        command{"Upload a file.", "[-aes] [-k] [-r rev] file destination", doPut},
	"restore":    command{"Restore a file to a previous revision.", "path revision", doRestore},
	"revisions":  command{"Get revisions of files.", "[-l] file destination", doRevisions},
	"search":     command{"Search files.", "[-a] [-l] [-m limit] path \"query words\"", doSearch},
	"shares":     command{"Share files.", "[-o] file [files...]", doShare},
	"thumbnails": command{"Download a thumbnail.", "[-s size] [-f format] files destination", doThumbnails},
}

func usage(name string) {
	fmt.Fprintf(os.Stderr, "Usage: %s command command_arguments\n", name)
	fmt.Fprintf(os.Stderr, "       Use help command to list available commands\n")
	fmt.Fprintf(os.Stderr, "       Use command -h to get help for commands accepting options\n")
	os.Exit(1)
}

func main() {
	var err error
	var db *dropbox.Dropbox
	var config ConfigFile

	if len(os.Args) < 2 {
		usage(os.Args[0])
	}
	commands["help"] = command{"Show this help message", "", doHelp}

	db = dropbox.NewDropbox()
	_ = config.Read(configFilename)
	db.SetAppInfo(appKey, appSecret)
	if len(config.Token) == 0 {
		if err = db.Auth(); err != nil {
			fmt.Println(err)
			return
		}
		config.Token = db.AccessToken()
		config.Write(configFilename)
	} else {
		db.SetAccessToken(config.Token)
	}
	if cmd, ok := commands[os.Args[1]]; ok {
		if err = cmd.Func(&config, db, os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	} else {
		fmt.Fprintf(os.Stderr, "Unknown command '%s'\n", os.Args[1])
		usage(os.Args[0])
	}
	if config.changed {
		config.Write(configFilename)
	}
}
