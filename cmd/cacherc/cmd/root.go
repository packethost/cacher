// Copyright © 2018 packet.net

package cmd

import (
	"fmt"
	"os"

	"github.com/packethost/cacher/client"
	"github.com/packethost/cacher/protos/cacher"
	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "cacherc",
	Short: "cacher client",
}

func connectGRPC(facility string, err error) cacher.CacherClient {
	if err != nil {
		panic(err)
	}
	c, err := client.New(facility)
	if err != nil {
		panic(err)
	}
	return c
}

func verifyUUIDs(args []string) error {
	if len(args) < 1 {
		return errors.New("requires at least one id")
	}
	for _, arg := range args {
		if _, err := uuid.FromString(arg); err != nil {
			return fmt.Errorf("invalid uuid: %s", arg)
		}
	}
	return nil
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "facility", "f", "", "used to build grcp and http urls")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}
