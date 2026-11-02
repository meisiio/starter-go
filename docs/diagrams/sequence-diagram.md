# نمودار توالی (Sequence Diagram) - تعامل سرویس ۶ به عنوان Consumer

این نمودار توالی تعامل سرویس ۶ (Reconciliation & Exceptions) را به عنوان **Consumer** در فراخوانی سرویس ۵ (Clearing & Settlement Gateway) به عنوان **Provider** برای دریافت گزارش‌های تسویه روزانه طبق نیازمندی‌های تمرین جلسه ۴ نشان می‌دهد.

## مشخصات اندپوینت فراخوانی‌شده
- **مسیر (Path):** `GET /api/v1/settlement-reports?date=2026-11-02`
- **سرآیند الزامی:** `X-Correlation-ID` (جهت ردیابی توزیع‌شده مطابق استاندارد GEN-08)
- **سناریوهای مدل‌شده در بلوک `alt`:**
  1. **مسیر موفق (200 OK):** دریافت فایل گزارش تسویه روزانه، مقایسه با اسناد دفترکل، و ثبت موفق فرآیند تطبیق.
  2. **مسیر خطای ۴۰۴ (404 Not Found):** عدم وجود فایل تسویه برای تاریخ درخواستی، دریافت قالب خطای RFC 7807، و انتقال وضعیت به `PENDING` جهت تلاش مجدد.
  3. **مسیر خطای ۵xx / اتمام مهلت زمانی (Timeout / 5xx):** قطعی سرویس ارائه‌دهنده یا عدم پاسخ در مهلت زمانی ۵ ثانیه، ثبت خطا در لاگ با کد `UPSTREAM_TIMEOUT` و تغییر وضعیت به `FAILED`.

---

```mermaid
sequenceDiagram
    autonumber
    actor Auditor as ناظر بانکی / سیستم زمان‌بندی
    participant S6 as S6 Reconciliation (Consumer)
    participant S5 as S5 Settlement (Provider)
    participant DB as S6 PostgreSQL

    Auditor->>S6: فراخوانی اجرای تطبیق روزانه (تاریخ: 2026-11-02)
    S6->>DB: درج رکورد reconciliation_jobs (وضعیت: RUNNING)
    
    S6->>S5: GET /api/v1/settlement-reports?date=2026-11-02<br/>Header: X-Correlation-ID: 0192a7e5-3b0d-7c55...
    
    alt ۱. مسیر موفق (200 OK)
        S5-->>S6: 200 OK (SettlementReport JSON شامل تراکنش‌های تسویه‌شده)
        S6->>S6: مقایسه رکوردهای تسویه با اسناد دریافتی از دفترکل (S2)
        S6->>DB: به‌روزرسانی وضعیت پرونده‌ها و ذخیره در exceptions
        S6->>DB: به‌روزرسانی reconciliation_jobs (وضعیت: COMPLETED)
        S6-->>Auditor: 201 Created (خلاصه فرآیند تطبیق و تعداد مغایرت‌های کشف‌شده)
        
    else ۲. مسیر خطای منبع یافت نشد (404 Not Found)
        S5-->>S6: 404 Not Found (Problem Details: کد NOT_FOUND)
        Note over S6: گزارش تسویه برای تاریخ انتخابی تولید نشده است
        S6->>DB: به‌روزرسانی وضعیت به PENDING جهت پردازش در چرخه بعدی
        S6-->>Auditor: 404 Problem Details (گزارش تسویه یافت نشد)
        
    else ۳. مسیر خطای سرور ارائه‌دهنده یا اتمام مهلت (503 / 504 / Timeout)
        S5-->>S6: 504 Gateway Timeout / 503 Service Unavailable
        Note over S6: سپری شدن مهلت پاسخ (Timeout)
        S6->>DB: به‌روزرسانی وضعیت به FAILED و ثبت خطا در لاگ ساخت‌یافته
        S6-->>Auditor: 504 Problem Details (کد UPSTREAM_TIMEOUT)
    end
```
