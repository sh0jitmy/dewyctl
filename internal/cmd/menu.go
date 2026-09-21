// Copyright (c) 2026 sh0jitmy <shjtmy@gmail.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// RunMenu launches the interactive operations menu
func RunMenu() error {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Println("")
		fmt.Println("============================================================")
		fmt.Println("           dewyctl - Dewy Operations & Toolkit")
		fmt.Println("============================================================")
		fmt.Println(" [作成支援 (プロジェクト初期化)]")
		fmt.Println("  1) init           GoReleaser / tagpr / GitHub Actions / systemd の生成支援")
		fmt.Println("")
		fmt.Println(" [設定・シークレット]")
		fmt.Println("  2) config         S3 認証情報・バケット対話的設定 (SOPS暗号化)")
		fmt.Println("")
		fmt.Println(" [ゼロダウンタイム E2E 自動検証 (ローカル & CI 両対応)]")
		fmt.Println("  3) test           E2E 自動テスト (v1配信 -> Dewy起動 -> v2リリース -> 切替検証)")
		fmt.Println("  4) server         Dewy サーバーを直接起動 (未インストール時は自動DL)")
		fmt.Println("  5) release        GoReleaser でS3へリリース (SOPS認証情報を自動注入)")
		fmt.Println("  6) push           バイナリを手動ビルドしてS3へ直接PUT")
		fmt.Println("")
		fmt.Println(" [Docker 検証 (systemd環境)]")
		fmt.Println("  7) docker all     一括実行 (Docker起動 + Ansible展開 + push + E2E検証)")
		fmt.Println("  8) docker setup   Dockerコンテナ起動 + Ansible展開 (Dewyデーモン起動)")
		fmt.Println("  9) docker verify  コンテナのE2Eテスト (:8080)")
		fmt.Println(" 10) docker logs    Dewy systemdログの表示")
		fmt.Println(" 11) docker clean   コンテナ停止・削除")
		fmt.Println("")
		fmt.Println(" [診断]")
		fmt.Println(" 12) doctor         S3疎通・依存環境のヘルスチェック")
		fmt.Println("  q) quit           終了")
		fmt.Println("============================================================")
		fmt.Print("Select option [1-12, q]: ")

		input, err := reader.ReadString('\n')
		if err != nil {
			return nil
		}
		choice := strings.TrimSpace(input)

		switch choice {
		case "1":
			if err := RunInit(nil); err != nil {
				fmt.Printf("[!] Error in init: %v\n", err)
			}
		case "2":
			if err := RunSopsConfig(nil); err != nil {
				fmt.Printf("[!] Error in config: %v\n", err)
			}
		case "3":
			if err := RunTest(nil); err != nil {
				fmt.Printf("[!] Error in test: %v\n", err)
			}
		case "4":
			if err := RunServer(nil); err != nil {
				fmt.Printf("[!] Error in server: %v\n", err)
			}
		case "5":
			if err := RunRelease(nil); err != nil {
				fmt.Printf("[!] Error in release: %v\n", err)
			}
		case "6":
			fmt.Print("Enter release version [v0.1.0]: ")
			v, _ := reader.ReadString('\n')
			v = strings.TrimSpace(v)
			if v == "" {
				v = "v0.1.0"
			}
			if err := RunPush([]string{"--version", v}); err != nil {
				fmt.Printf("[!] Error in push: %v\n", err)
			}
		case "7":
			fmt.Print("Enter release version [v0.1.0]: ")
			v, _ := reader.ReadString('\n')
			v = strings.TrimSpace(v)
			if v == "" {
				v = "v0.1.0"
			}
			if err := RunDocker([]string{"all", "--version", v}); err != nil {
				fmt.Printf("[!] Error in docker all: %v\n", err)
			}
		case "8":
			if err := RunDocker([]string{"setup"}); err != nil {
				fmt.Printf("[!] Error in docker setup: %v\n", err)
			}
		case "9":
			fmt.Print("Enter expected version [v0.1.0]: ")
			v, _ := reader.ReadString('\n')
			v = strings.TrimSpace(v)
			if v == "" {
				v = "v0.1.0"
			}
			if err := RunDocker([]string{"verify", "--version", v}); err != nil {
				fmt.Printf("[!] Error in docker verify: %v\n", err)
			}
		case "10":
			if err := RunDocker([]string{"logs"}); err != nil {
				fmt.Printf("[!] Error in docker logs: %v\n", err)
			}
		case "11":
			if err := RunDocker([]string{"clean"}); err != nil {
				fmt.Printf("[!] Error in docker clean: %v\n", err)
			}
		case "12":
			if err := RunDoctor(nil); err != nil {
				fmt.Printf("[!] Error in doctor: %v\n", err)
			}
		case "q", "Q", "exit", "quit":
			fmt.Println("Exiting dewyctl. Goodbye!")
			return nil
		default:
			fmt.Printf("Invalid option: '%s'. Please select 1-12 or q.\n", choice)
		}
	}
}
