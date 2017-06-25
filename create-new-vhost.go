package main

import (
	"fmt"
	"github.com/pborman/getopt/v2"
	"github.com/termie/go-shutil"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
)

var (
	domain          = ""
	template_dir    = "/etc/create-new-vhost/template-directory"
	htaccess_file   = "/etc/publickeys/authpwd/authpwd"
	destination_dir = "/srv/http/"
	http_conf       = "http.conf"
	http_ssl_conf   = "http-ssl.conf"
	sites_available = "/etc/apache2/sites-available"
	www_user        = "www-data"
	www_group       = "www-data"
	user_id         int
	group_id        int
)

func init() {
	getopt.FlagLong(&domain, "domain", 0, "domain name for vhost")
	getopt.FlagLong(&template_dir, "template-dir", 0, "template directory for this script").SetOptional()
	getopt.FlagLong(&destination_dir, "destination-dir", 0, "destination directory for this script").SetOptional()
	getopt.FlagLong(&htaccess_file, "htaccess-file", 0, "path to htaccess to be used").SetOptional()
	getopt.FlagLong(&www_user, "www-user", 0, "webserver user").SetOptional()
	getopt.FlagLong(&www_group, "www-group", 0, "webserver group").SetOptional()
	getopt.FlagLong(&sites_available, "sites-available", 0, "directory to symlink virtualhost configuration to").SetOptional()

}

func fetch_user_group_id() {
	usr, err := user.Lookup(www_user)
	if err != nil {
		log.Fatal(err)
		os.Exit(8)
	}

	grp, _ := user.LookupGroup(www_group)
	if err != nil {
		log.Fatal(err)
		os.Exit(8)
	}

	user_id, _ = strconv.Atoi(usr.Uid)
	group_id, _ = strconv.Atoi(grp.Gid)
}

func check_for_root() {
	if os.Geteuid() != 0 {
		fmt.Println("Need to run as root")
		os.Exit(1)
	}
}

func print_arguments_summary() {
	format_str := `
Domain:             %v
destination-dir:    %v
www-user:           %v
www-group:          %v
`
	fmt.Printf(format_str, domain, destination_dir, www_user, www_group)
	fmt.Print("To you want to continue [n/Y] ")

	var response string
	_, err := fmt.Scanln(&response)
	if err != nil {
		log.Fatal(err)
	}

	if response != "y" {
		os.Exit(5)
	}
}

func main() {
	check_for_root()
	getopt.Parse()
	if domain == "" {
		fmt.Println("No domain given!")
		getopt.Usage()
		os.Exit(1)
	}

	args := getopt.Args()
	if len(args) > 0 && args[0] == "redo-ssl" {
		fmt.Println("redo ssl")
	} else {
		fetch_user_group_id()

		print_arguments_summary()

		create_new_virtualhost()
	}

}

func create_new_virtualhost() {
	full_destination := destination_dir + domain
	err := shutil.CopyTree(template_dir, full_destination, nil)
	if err != nil {
		switch err.(type) {
		case shutil.AlreadyExistsError:
			log.Fatal("Virtualhost directory already exists, aborting: ")
			os.Exit(7)
		}
	}

	files, err := ioutil.ReadDir(full_destination)
	if err != nil {
		log.Fatal(err)
		os.Exit(8)
	}

	// change mode directory itself
	change_ownership(full_destination)

	modify_http_conf(filepath.Join(full_destination, http_conf))
	modify_http_conf(filepath.Join(full_destination, http_ssl_conf))
	activate_vhost(filepath.Join(full_destination, http_conf))

	for _, file := range files {
		joined_path := filepath.Join(full_destination, file.Name())

		change_ownership(joined_path)
	}
}

func activate_vhost(vhost_config string) {
	err := os.Symlink(vhost_config, filepath.Join(sites_available, domain+".conf"))
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	a2ensite_path, err := exec.LookPath("a2ensite")
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	a2ensite_cmd := exec.Command(a2ensite_path, domain)
	err = a2ensite_cmd.Run()
	if err != nil {
		log.Fatal(err)
		os.Exit(254)
	}
}

func modify_http_conf(http_conf_file string) {
	contents, err := ioutil.ReadFile(http_conf_file)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
	new_contents := strings.Replace(string(contents[:]), "%DOMAIN%", domain, -1)

	err = ioutil.WriteFile(http_conf_file, []byte(new_contents), 0775)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
}

func change_ownership(dst string) {
	// change ownership of domain directory to webserver user and group
	err := os.Chown(dst, user_id, group_id)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	// change mode of domain directory to webserver user and group
	err = os.Chmod(dst, 0775)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
}
