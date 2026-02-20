// Copyright 2025 Erst Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"os"
	"sync"

	"github.com/dotandev/hintents/internal/compare"
	"github.com/dotandev/hintents/internal/config"
	"github.com/dotandev/hintents/internal/errors"
	"github.com/dotandev/hintents/internal/rpc"
	"github.com/dotandev/hintents/internal/simulator"
	"github.com/spf13/cobra"
)

var (
	compareWasmFlag string
	compareArgsFlag []string
)

var compareCmd = &cobra.Command{
	Use:   "compare <transaction-hash>",
	Short: "Compare on-chain vs local WASM execution",
	Long: `Replay a transaction against both on-chain WASM and a local WASM file,
then show side-by-side differences in events, logs, and execution results.

This is essential for "What broke when I updated?" debugging.

Example:
  erst compare <tx-hash> --wasm ./target/wasm32-unknown-unknown/release/my_contract.wasm
  erst compare --network testnet <tx-hash> --wasm ./contract.wasm`,
	Args: cobra.ExactArgs(1),
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if compareWasmFlag == "" {
			return fmt.Errorf("--wasm flag is required")
		}
		if _, err := os.Stat(compareWasmFlag); err != nil {
			return fmt.Errorf("WASM file not found: %s: %w", compareWasmFlag, err)
		}
		switch rpc.Network(networkFlag) {
		case rpc.Testnet, rpc.Mainnet, rpc.Futurenet:
			return nil
		default:
			return errors.WrapInvalidNetwork(networkFlag)
		}
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		txHash := args[0]

		token := rpcTokenFlag
		if token == "" {
			token = os.Getenv("ERST_RPC_TOKEN")
		}
		if token == "" {
			cfg, err := config.LoadConfig()
			if err == nil && cfg.RPCToken != "" {
				token = cfg.RPCToken
			}
		}

		opts := []rpc.ClientOption{
			rpc.WithNetwork(rpc.Network(networkFlag)),
			rpc.WithToken(token),
		}
		if rpcURLFlag != "" {
			opts = append(opts, rpc.WithHorizonURL(rpcURLFlag))
		}

		client, err := rpc.NewClient(opts...)
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}

		fmt.Printf("Comparing execution for transaction: %s\n", txHash)
		fmt.Printf("Network: %s\n", networkFlag)
		fmt.Printf("Local WASM: %s\n\n", compareWasmFlag)

		// Fetch transaction details
		txResp, err := client.GetTransaction(ctx, txHash)
		if err != nil {
			return fmt.Errorf("failed to fetch transaction: %w", err)
		}

		// Initialize simulator
		runner, err := simulator.NewRunner("", verbose)
		if err != nil {
			return fmt.Errorf("failed to initialize simulator: %w", err)
		}

		keys, err := extractLedgerKeys(txResp.ResultMetaXdr)
		if err != nil {
			return fmt.Errorf("failed to extract ledger keys: %w", err)
		}

		ledgerEntries, err := client.GetLedgerEntries(ctx, keys)
		if err != nil {
			return fmt.Errorf("failed to fetch ledger entries: %w", err)
		}

		// Run two passes in parallel: on-chain and local-WASM replay.
		fmt.Printf("Running on-chain and local simulations...\n")
		onChainReq := &simulator.SimulationRequest{
			EnvelopeXdr:   txResp.EnvelopeXdr,
			ResultMetaXdr: txResp.ResultMetaXdr,
			LedgerEntries: ledgerEntries,
		}
		localWasmPath := compareWasmFlag
		localReq := &simulator.SimulationRequest{
			EnvelopeXdr:   txResp.EnvelopeXdr,
			ResultMetaXdr: txResp.ResultMetaXdr,
			LedgerEntries: ledgerEntries,
			WasmPath:      &localWasmPath,
			MockArgs:      &compareArgsFlag,
		}

		var (
			wg          sync.WaitGroup
			onChainResp *simulator.SimulationResponse
			localResp   *simulator.SimulationResponse
			onChainErr  error
			localSimErr error
		)

		wg.Add(2)
		go func() {
			defer wg.Done()
			onChainResp, onChainErr = runner.Run(onChainReq)
		}()
		go func() {
			defer wg.Done()
			localResp, localSimErr = runner.Run(localReq)
		}()
		wg.Wait()

		if onChainErr != nil {
			return fmt.Errorf("on-chain simulation failed: %w", onChainErr)
		}
		if localSimErr != nil {
			return fmt.Errorf("local WASM simulation failed: %w", localSimErr)
		}

		// Compare results
		diff := compare.CompareResults(onChainResp, localResp)

		fmt.Printf("\n=== Comparison Results ===\n")
		fmt.Print(diff.FormatSideBySide())

		return nil
	},
}

func init() {
	compareCmd.Flags().StringVar(&compareWasmFlag, "wasm", "", "Path to local WASM file to compare against on-chain version (required)")
	compareCmd.Flags().StringSliceVar(&compareArgsFlag, "args", []string{}, "Mock arguments for local WASM replay")
	compareCmd.Flags().StringVarP(&networkFlag, "network", "n", string(rpc.Mainnet), "Stellar network to use (testnet, mainnet, futurenet)")
	compareCmd.Flags().StringVar(&rpcURLFlag, "rpc-url", "", "Custom Horizon RPC URL to use")

	rootCmd.AddCommand(compareCmd)
}
