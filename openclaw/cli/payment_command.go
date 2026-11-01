package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/futugyou/openclaw/payments"
	"github.com/futugyou/openclaw/util"
)

func PaymentCommandRun(args []string) (int, error) {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		printPaymentCommandHelp()
		return 0, nil
	}

	parsed := CliArgsParse(args)
	m := min(len(parsed.Positionals), 2)
	command := strings.Join(parsed.Positionals[0:m], " ")
	if len(parsed.Positionals) == 1 {
		command = parsed.Positionals[0]
	}
	client, err := createClient(parsed)
	if err != nil {
		return 1, err
	}

	ctx := context.Background()
	provider := util.Deref(parsed.GetOption("--provider"))
	environment := util.Deref(parsed.GetOption("--environment"))
	isjson := parsed.HasFlag("--json")

	switch command {
	case "setup":
		status, err := client.GetPaymentSetupStatus(ctx, provider)
		if err != nil {
			return 1, err
		}
		paymentCommandWrite(*status, isjson, paymentCommandTextSetup)
		return 0, nil
	case "funding list":
		items, err := client.ListPaymentFundingSources(ctx, provider, environment)
		if err != nil {
			return 1, err
		}
		paymentCommandWrite(items, isjson, paymentCommandTextFunding)
		return 0, nil
	case "virtual-card issue":
		currency := util.Deref(parsed.GetOption("--currency"))
		if currency == "" {
			currency = "USD"
		}
		merchant := util.Deref(parsed.GetOption("--merchant"))
		var amount int64 = 0
		if p, err := strconv.ParseInt(util.Deref(parsed.GetOption("--amount-minor")), 10, 64); err == nil {
			amount = p
		}
		var minutes int64 = 30
		if p, err := strconv.ParseInt(util.Deref(parsed.GetOption("--valid-minutes")), 10, 64); err == nil {
			minutes = p
		}
		request := payments.VirtualCardRequest{
			ProviderId:      provider,
			FundingSourceId: util.Deref(parsed.GetOption("--funding-source")),
			MerchantName:    merchant,
			MerchantUrl:     util.Deref(parsed.GetOption("--merchant-url")),
			AmountMinor:     amount,
			Currency:        currency,
			Purpose:         util.Deref(parsed.GetOption("--purpose")),
			ValidUntilUtc:   new(time.Now().Add(time.Duration(minutes) * time.Minute)),
			Environment:     environment,
		}
		handle, err := client.IssueVirtualCard(ctx, request, parsed.HasFlag("--yes"))
		if err != nil {
			return 1, err
		}
		paymentCommandWrite(*handle, isjson, paymentCommandTextHandle)
		return 0, nil
	case "status":
		id := util.Deref(parsed.GetOption("--id"))
		status, err := client.GetPaymentStatus(ctx, id, provider, environment)
		if err != nil {
			return 1, err
		}
		paymentCommandWrite(*status, isjson, paymentCommandTextStatus)
		return 0, nil
	case "execute":
		protocol := util.Deref(parsed.GetOption("--protocol"))
		if protocol == "" {
			protocol = "http-402"
		}
		currency := util.Deref(parsed.GetOption("--currency"))
		if currency == "" {
			currency = "USD"
		}
		var amount int64 = 0
		if p, err := strconv.ParseInt(util.Deref(parsed.GetOption("--amount-minor")), 10, 64); err == nil {
			amount = p
		}
		request := payments.MachinePaymentRequest{
			ProviderId:      provider,
			FundingSourceId: util.Deref(parsed.GetOption("--funding-source")),
			Environment:     environment,
			Challenge: payments.MachinePaymentChallenge{
				ChallengeId:  util.Deref(parsed.GetOption("--challenge-id")),
				Protocol:     protocol,
				ResourceUrl:  util.Deref(parsed.GetOption("--resource-url")),
				MerchantName: util.Deref(parsed.GetOption("--merchant")),
				AmountMinor:  amount,
				Currency:     currency,
				ProviderId:   provider,
			},
		}
		result, err := client.ExecuteMachinePayment(ctx, request, parsed.HasFlag("--yes"))
		if err != nil {
			return 1, err
		}
		paymentCommandWrite(*result, isjson, paymentCommandTextMachinePayment)
		return 0, nil
	}

	printPaymentCommandHelp()
	return 0, nil
}

func paymentCommandTextSetup(status payments.PaymentSetupStatus) {
	fmt.Printf("%s: %s mode=%s installed=%t\n", status.ProviderId, status.Status, status.Mode, status.Installed)
	if status.Version != "" {
		fmt.Printf("version: %s\n", status.Version)
	}
	if status.Message != "" {
		fmt.Println(status.Message)
	}
}

func paymentCommandWrite[T any](value T, isjson bool, text func(T)) {
	if isjson {
		d, _ := json.Marshal(value)
		fmt.Println(string(d))
	} else {
		text(value)
	}
}
func paymentCommandTextFunding(items []payments.FundingSource) {
	for _, item := range items {
		fmt.Printf("%s\t%s\t%s\tlast4=%s\tavailable=%t\n", item.FundingSourceId, item.ProviderId, item.DisplayName, item.Last4, item.Available)
	}
}

func paymentCommandTextMachinePayment(result payments.MachinePaymentResult) {
	fmt.Printf("%s\t%s\t%s\tmerchant=%s\tamount=%d %s\n", result.PaymentId, result.ProviderId, result.Status, result.MerchantName, result.AmountMinor, result.Currency)
}

func paymentCommandTextHandle(handle payments.VirtualCardHandle) {
	fmt.Printf("%s\t%s\tlast4=%s\tmerchant=%s\tvalidUntil=%s\tstatus=%s\n", handle.HandleId, handle.ProviderId, handle.Last4, handle.TargetMerchantName, handle.ValidUntilUtc, handle.Status)
}

func paymentCommandTextStatus(status payments.PaymentStatus) {
	fmt.Printf("%s\t%s\t%s\tmerchant=%s\tamount=%d %s\n", status.PaymentId, status.ProviderId, status.Status, status.MerchantName, status.AmountMinor, status.Currency)
}

func printPaymentCommandHelp() {
	fmt.Println(
		` 
            openclaw payment

            Usage:
              openclaw payment setup [--provider <id>] [--json]
              openclaw payment funding list [--provider <id>] [--test|--environment live] [--json]
              openclaw payment virtual-card issue --merchant <name> --amount-minor <n> [--currency USD] [--funding-source <id>] [--provider <id>] [--test|--environment live] [--yes] [--json]
              openclaw payment execute --amount-minor <n> [--merchant <name>] [--resource-url <url>] [--currency USD] [--provider <id>] [--test|--environment live] [--yes] [--json]
              openclaw payment status --id <payment-or-handle-id> [--provider <id>] [--json]

            Notes:
              - Money-moving commands default to test mode. Use --environment live --yes for live payment attempts.
              - Commands talk to the running gateway so handles remain usable by browser sentinel substitution.
              - Output is allow-listed safe metadata only.
             `)
}
