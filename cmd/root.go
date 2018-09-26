package cmd

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/a8m/mark"
	"github.com/gernest/front"
	"github.com/lbryio/lbry.go/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/tyler-sommer/stick"
)

var confFile string

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&confFile, "conf", ".merlin.yml", "config file (default is $CWD/.merlin.yml)")
}

var rootCmd = &cobra.Command{
	Use:   "merlin",
	Short: "Merlin builds my site automagically",
	Run:   build,
}

func build(cmd *cobra.Command, args []string) {
	pwd, err := os.Getwd()
	check(err)
	check(buildSite(pwd + "/test"))
}

func buildSite(dir string) error {
	twig := stick.New(stick.NewFilesystemLoader(dir + "/_layouts"))
	m := front.NewMatter()
	m.Handle("---", front.YAMLHandler)

	target := dir + "/out"
	check(os.RemoveAll(target))
	check(os.Mkdir(target, 0755))

	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if path == target || strings.HasPrefix(path, target+"/") {
			return nil
		}

		relPath := strings.TrimLeft(path[len(dir):], "/")

		if relPath == "" || strings.HasPrefix(relPath, "_") {
			return nil
		}

		dst := target + "/" + strings.TrimSuffix(relPath, filepath.Ext(relPath)) + ".html"
		err = os.MkdirAll(filepath.Dir(dst), 0755)
		if err != nil {
			return err
		}

		var contentType string

		switch filepath.Ext(relPath) {
		case ".html":
			contentType = "html"
			copyFileContents(path, dst)

		case ".md":
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()

			fm, body, err := m.Parse(f)
			if err != nil {
				return err
			}

			html := mark.Render(body)

			layout := "post"
			if l, ok := fm["layout"]; ok {
				layout, ok = l.(string)
				if !ok {
					return errors.Err(`frontmatter: "layout" must be a string`)
				}
			}
			contentType = "markdown " + layout

			buf := &strings.Builder{}
			err = twig.Execute(layout+".twig", buf, map[string]stick.Value{
				"content": html,
			})
			if err != nil {
				return err
			}

			err = ioutil.WriteFile(dst, []byte(buf.String()), 0664)
			if err != nil {
				return err
			}

		default:
			contentType = "unsupported"
		}

		fmt.Printf("->> %s (%s)\n", relPath, contentType)
		return nil
	})
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func initConfig() {
	viper.SetConfigFile(confFile)
	if err := viper.ReadInConfig(); err != nil {
		if os.IsNotExist(err) {
			// go with defaults?
		}
		fmt.Println("Can't read config:", err)
		os.Exit(1)
	}
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return true, err
}

func copyFileContents(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return
	}

	defer func() {
		cerr := out.Close()
		if err == nil {
			err = cerr
		}
	}()

	if _, err = io.Copy(out, in); err != nil {
		return
	}

	err = out.Sync()
	return
}
