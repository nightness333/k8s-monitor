package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Очищает накопленные данные мониторинга",
	Run: func(cmd *cobra.Command, args []string) {
		file, _ := cmd.Flags().GetString("file")

		data, err := os.OpenFile(file, os.O_TRUNC|os.O_WRONLY, 0644)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Println("Файл данных не найден, нечего очищать.")
			} else {
				fmt.Println("Ошибка при очистке файла данных:", err)
			}
			return
		}
		defer data.Close()

		fmt.Println("Данные успешно очищены.")
	},
}

func init() {
	rootCmd.AddCommand(resetCmd)
	resetCmd.Flags().StringP("file", "f", "/data/output.csv", "Файл с метриками")
}
