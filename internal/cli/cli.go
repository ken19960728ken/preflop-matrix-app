package cli

import (
	"fmt"
	"os"
)

// Run 執行命令列模式並回傳 exit code。
func Run(args []string) int {
	fmt.Fprintln(os.Stderr, "not implemented")
	return 2
}
