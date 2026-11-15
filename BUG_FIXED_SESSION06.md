# BUG_FIXED_SESSION06.md

- سرویس و اعضای تیم: S6-RCN (Go) 
    سیدمحمدرضا حمیدی 40331062
    محمدحسین مرتضائی 40331047
    رضا شالچیان تبریزی 40331061 
- مخزن تمرین: https://github.com/AUT-SE1/LabBank-S6-Session06

## رفتار اشتباه و علت اصلی

**بازتولید:**

| مورد | مقدار |
|---|---|
| ورودی ۱ (پاسخ جعلی S2) | `total_debit=100000000, total_credit=95000000, is_balanced=true` |
| ورودی ۲ (پاسخ جعلی S2) | `total_debit=100000000, total_credit=100000000, is_balanced=false` |
| مراحل بازتولید | `go test -run TrialBalance -v -count=1 ./...` (سرور S2 با `httptest.NewServer` شبیه‌سازی شده) |
| نتیجه ی واقعی (قبل از اصلاح) | `FetchTrialBalance` در هر دو ورودی `nil` برمیگرداند، یعنی پاسخ را میپذیرد |
| نتیجه‌ی مورد انتظار | در هر دو ورودی خطای `ErrTrialBalanceFailed` (طبق راهنما: نقض هر یک از دو معیار به تنهایی باید پاسخ را رد کند) |

**چرا تست قبلی نمی دید:** `TestS2Client_FetchTrialBalance_Unbalanced` هر دو نشانه را همزمان نامعتبر می‌کرد.

**شواهد دیباگر:** <با Delve یا VS Code روی خط شرط در `FetchTrialBalance` breakpoint گذاشتیم؛ برای ورودی ۱ مقدار `tb.IsBalanced=true` و `tb.TotalDebit != tb.TotalCredit` برابر true بود، ولی `!tb.IsBalanced && ...` برابر false شد و خطا برنگشت. اسکرین شات/خروجی `print tb` و `bt` را اینجا بگذارید>

**علت اصلی:** شرط رد پاسخ با عملگر منطقی اشتباه نوشته شده بود:

```
if !tb.IsBalanced && tb.TotalDebit != tb.TotalCredit {
```

پاسخ وقتی معتبر است که `IsBalanced && debit == credit`. نقیض آن `!IsBalanced || debit != credit` است. با `&&` پاسخ فقط وقتی رد می‌شد که هر دو نشانه هم‌زمان نامعتبر باشند.

## اصلاح انجام شده

تغییر یک‌خطی در `FetchTrialBalance` (حداقل تغییر لازم):

```
if !tb.IsBalanced || tb.TotalDebit != tb.TotalCredit {
```

رفتار قبل: ورودی ۱ و ۲ پذیرفته می شد. رفتار بعد: هر دو با `ErrTrialBalanceFailed` رد می شوند. کدهای خطا، پیام‌ها و وضعیت HTTP تغییری نکردند.


## تست‌ها و نتیجه اجرا

- تست پیش از اصلاح: `go test -run TrialBalance -v -count=1 ./...` — <FAIL برای `FlagTrueButTotalsDiffer_IsRejected`، `FlagFalseButTotalsEqual_IsRejected` و دو ردیف ماتریس؛ خروجی کوتاه>
- تست پس از اصلاح: همان دستور — <PASS؛ خروجی کوتاه>
- تست‌های قفل‌کننده‌ی رفتار (قبل و بعد از بازآرایی): `go test -run "ProblemJSON|NonProblemBody" -v -count=1 ./...` — <PASS در هر دو بار>
- نتیجه کل تست‌ها: `go test ./... -count=1` — <خروجی؛ مثلاً `ok example.com/softwarelab/starter ...`. اگر تستی به دلیل نبود دیتابیس/Docker/تنظیمات اجرا نشد، نام تست، علت و متن خطا را اینجا بنویسید>

-------------

-- before

┌────(Hossein@Meow)-[~/Desktop/LabBank-S6-Session06][main] 
└─$ go test -run TrialBalance -v -count=1 ./... 2>&1 | tee ~/before-fix.txt
=== RUN   TestS2Client_FetchTrialBalance
--- PASS: TestS2Client_FetchTrialBalance (0.00s)
=== RUN   TestS2Client_FetchTrialBalance_Unbalanced
--- PASS: TestS2Client_FetchTrialBalance_Unbalanced (0.00s)
=== RUN   TestS2Client_FetchTrialBalance_FlagTrueButTotalsDiffer_IsRejected
    s2client_trial_balance_test.go:38: is_balanced=true with debit != credit must be rejected, got nil error
--- FAIL: TestS2Client_FetchTrialBalance_FlagTrueButTotalsDiffer_IsRejected (0.00s)
=== RUN   TestS2Client_FetchTrialBalance_FlagFalseButTotalsEqual_IsRejected
    s2client_trial_balance_test.go:57: is_balanced=false with debit == credit must be rejected, got nil error
--- FAIL: TestS2Client_FetchTrialBalance_FlagFalseButTotalsEqual_IsRejected (0.00s)
=== RUN   TestS2Client_FetchTrialBalance_ValidationMatrix
=== RUN   TestS2Client_FetchTrialBalance_ValidationMatrix/totals_equal_and_flag_true_->_accepted
=== RUN   TestS2Client_FetchTrialBalance_ValidationMatrix/totals_differ_and_flag_true_->_rejected
    s2client_trial_balance_test.go:87: error = <nil>, want ErrTrialBalanceFailed
=== RUN   TestS2Client_FetchTrialBalance_ValidationMatrix/totals_equal_and_flag_false_->_rejected
    s2client_trial_balance_test.go:87: error = <nil>, want ErrTrialBalanceFailed
=== RUN   TestS2Client_FetchTrialBalance_ValidationMatrix/totals_differ_and_flag_false_->_rejected
--- FAIL: TestS2Client_FetchTrialBalance_ValidationMatrix (0.00s)
    --- PASS: TestS2Client_FetchTrialBalance_ValidationMatrix/totals_equal_and_flag_true_->_accepted (0.00s)
    --- FAIL: TestS2Client_FetchTrialBalance_ValidationMatrix/totals_differ_and_flag_true_->_rejected (0.00s)
    --- FAIL: TestS2Client_FetchTrialBalance_ValidationMatrix/totals_equal_and_flag_false_->_rejected (0.00s)
    --- PASS: TestS2Client_FetchTrialBalance_ValidationMatrix/totals_differ_and_flag_false_->_rejected (0.00s)
FAIL
FAIL	example.com/softwarelab/starter	0.009s
?   	example.com/softwarelab/starter/docs	[no test files]
FAIL


--after


┌────(Hossein@Meow)-[~/Desktop/LabBank-S6-Session06][main!] 
└─$ go test -run TrialBalance -v -count=1 ./... 2>&1 | tee ~/after-fix.txt                                                                                              9m34s
=== RUN   TestS2Client_FetchTrialBalance
--- PASS: TestS2Client_FetchTrialBalance (0.00s)
=== RUN   TestS2Client_FetchTrialBalance_Unbalanced
--- PASS: TestS2Client_FetchTrialBalance_Unbalanced (0.00s)
=== RUN   TestS2Client_FetchTrialBalance_FlagTrueButTotalsDiffer_IsRejected
--- PASS: TestS2Client_FetchTrialBalance_FlagTrueButTotalsDiffer_IsRejected (0.00s)
=== RUN   TestS2Client_FetchTrialBalance_FlagFalseButTotalsEqual_IsRejected
--- PASS: TestS2Client_FetchTrialBalance_FlagFalseButTotalsEqual_IsRejected (0.00s)
=== RUN   TestS2Client_FetchTrialBalance_ValidationMatrix
=== RUN   TestS2Client_FetchTrialBalance_ValidationMatrix/totals_equal_and_flag_true_->_accepted
=== RUN   TestS2Client_FetchTrialBalance_ValidationMatrix/totals_differ_and_flag_true_->_rejected
=== RUN   TestS2Client_FetchTrialBalance_ValidationMatrix/totals_equal_and_flag_false_->_rejected
=== RUN   TestS2Client_FetchTrialBalance_ValidationMatrix/totals_differ_and_flag_false_->_rejected
--- PASS: TestS2Client_FetchTrialBalance_ValidationMatrix (0.00s)
    --- PASS: TestS2Client_FetchTrialBalance_ValidationMatrix/totals_equal_and_flag_true_->_accepted (0.00s)
    --- PASS: TestS2Client_FetchTrialBalance_ValidationMatrix/totals_differ_and_flag_true_->_rejected (0.00s)
    --- PASS: TestS2Client_FetchTrialBalance_ValidationMatrix/totals_equal_and_flag_false_->_rejected (0.00s)
    --- PASS: TestS2Client_FetchTrialBalance_ValidationMatrix/totals_differ_and_flag_false_->_rejected (0.00s)
PASS
ok  	example.com/softwarelab/starter	0.008s
?   	example.com/softwarelab/starter/docs	[no test files]

┌────(Hossein@Meow)-[~/Desktop/LabBank-S6-Session06][main!] 
└─$ go test ./... -count=1 -v 2>&1 | tee ~/all-tests-after-fix.txt
=== RUN   TestGetAccountExceptions_InvalidUUID
--- PASS: TestGetAccountExceptions_InvalidUUID (0.00s)
=== RUN   TestAddTwoNumbers
--- PASS: TestAddTwoNumbers (0.00s)
=== RUN   TestHealthEndpoint
--- PASS: TestHealthEndpoint (0.00s)
=== RUN   TestHealthEndpointRejectsPost
--- PASS: TestHealthEndpointRejectsPost (0.00s)
=== RUN   TestS2Client_FetchJournalEntries
--- PASS: TestS2Client_FetchJournalEntries (0.00s)
=== RUN   TestS2Client_FetchTrialBalance
--- PASS: TestS2Client_FetchTrialBalance (0.00s)
=== RUN   TestS2Client_FetchTrialBalance_Unbalanced
--- PASS: TestS2Client_FetchTrialBalance_Unbalanced (0.00s)
=== RUN   TestS2Client_ProblemDetailsError
--- PASS: TestS2Client_ProblemDetailsError (0.00s)
=== RUN   TestS2Client_FetchTrialBalance_FlagTrueButTotalsDiffer_IsRejected
--- PASS: TestS2Client_FetchTrialBalance_FlagTrueButTotalsDiffer_IsRejected (0.00s)
=== RUN   TestS2Client_FetchTrialBalance_FlagFalseButTotalsEqual_IsRejected
--- PASS: TestS2Client_FetchTrialBalance_FlagFalseButTotalsEqual_IsRejected (0.00s)
=== RUN   TestS2Client_FetchTrialBalance_ValidationMatrix
=== RUN   TestS2Client_FetchTrialBalance_ValidationMatrix/totals_equal_and_flag_true_->_accepted
=== RUN   TestS2Client_FetchTrialBalance_ValidationMatrix/totals_differ_and_flag_true_->_rejected
=== RUN   TestS2Client_FetchTrialBalance_ValidationMatrix/totals_equal_and_flag_false_->_rejected
=== RUN   TestS2Client_FetchTrialBalance_ValidationMatrix/totals_differ_and_flag_false_->_rejected
--- PASS: TestS2Client_FetchTrialBalance_ValidationMatrix (0.00s)
    --- PASS: TestS2Client_FetchTrialBalance_ValidationMatrix/totals_equal_and_flag_true_->_accepted (0.00s)
    --- PASS: TestS2Client_FetchTrialBalance_ValidationMatrix/totals_differ_and_flag_true_->_rejected (0.00s)
    --- PASS: TestS2Client_FetchTrialBalance_ValidationMatrix/totals_equal_and_flag_false_->_rejected (0.00s)
    --- PASS: TestS2Client_FetchTrialBalance_ValidationMatrix/totals_differ_and_flag_false_->_rejected (0.00s)
PASS
ok  	example.com/softwarelab/starter	0.016s
?   	example.com/softwarelab/starter/docs	[no test files]


----------


## فایل‌های تغییر یافته

| فایل | نوع تغییر |
|---|---|
| `s2client.go` | اصلاح شرط (`&&` به `\|\|`) و بازآرایی خطای HTTP به `newS2ProblemError` (دو کامیت جدا) |
| `s2client_trial_balance_test.go` | جدید: تست بازتولید باگ و ماتریس درستی |

## نتیجه نهایی

باگ (استفاده از AND به جای OR در اعتبارسنجی تراز آزمایشی) با تست مستقل بازتولید شد، با تغییر یک خطی رفع شد، تست بازتولید از FAIL به PASS رسید و تست های موجود دچار Regression نشدند. و بدون تغییر رفتار بیرونی انجام شد.
