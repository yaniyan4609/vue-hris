package httpapi

import "testing"

func TestAllocatePayrollLoanDeductionsDefersWholeInstallments(t *testing.T) {
	items := []payrollLoanCandidate{
		{LoanID: "loan-a", Seq: 1, amountCents: 50000},
		{LoanID: "loan-b", Seq: 1, amountCents: 30000},
		{LoanID: "loan-a", Seq: 2, amountCents: 20000},
		{LoanID: "loan-b", Seq: 2, amountCents: 10000},
	}
	planned, deferred, remaining := allocatePayrollLoanDeductions(items, 40000)
	if len(planned) != 2 || planned[0].LoanID != "loan-b" || planned[1].Seq != 2 || len(deferred) != 2 || deferred[0].Reason != "INSUFFICIENT_NET" || deferred[1].Reason != "PRIOR_INSTALLMENT_DEFERRED" || remaining != 0 {
		t.Fatalf("planned=%#v deferred=%#v remaining=%d", planned, deferred, remaining)
	}
	planned, deferred, remaining = allocatePayrollLoanDeductions(items[:1], 49999)
	if len(planned) != 0 || len(deferred) != 1 || remaining != 49999 {
		t.Fatalf("partial deduction occurred: planned=%#v deferred=%#v remaining=%d", planned, deferred, remaining)
	}
}
