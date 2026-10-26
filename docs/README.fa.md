# هستهٔ PG CORE / چیمرا

[فارسی](README.fa.md) | [English](README.en.md)

**وضعیت: اسکلت اولیه؛ آمادهٔ استفادهٔ تولیدی نیست.**

PG CORE هماهنگ‌کنندهٔ ماژولار موتورهای شبکه است. اعتبارسنجی پیکربندی، چرخهٔ عمر موتور، تشخیص محدود DNS/HTTPS، انتخاب مسیر در سطح اتصال و آداپترهای اولیهٔ sing-box و Xray دارد. این موارد به معنی VPN کامل و آمادهٔ تولید نیستند.

## قابلیت‌ها

- هستهٔ Go برای اعتبارسنجی، هماهنگی موتور و تشخیص شبکه
- آداپترهای اولیهٔ sing-box و Xray؛ نیازمند باینری نصب‌شده و آزمون اتصال واقعی
- تونل بومی TLS/WebSocket و پراکسی SOCKS5 محلی
- حالت‌های انتخاب نامزد CDN/SNI بر پایهٔ فهرست‌های ارائه‌شده؛ کشف زندهٔ عمومی انجام نمی‌شود
- Ghost: تحلیل محدود علائم DNS/HTTPS؛ خطا به‌تنهایی اثبات DPI نیست
- Ghost Plus (C): طبقه‌بندی محافظه‌کارانهٔ مشاهدات
- Ghost Ultra Plus (Kotlin و Python): گزارش‌سازی محلی و خروجی فارسی/انگلیسی
- API و تست‌های واحد برای بخش‌های پیاده‌سازی‌شده

## اجرای بررسی Ghost

```sh
go run ./cmd/chimera --ghost example.com --samples 5
python tools/ghost_ultra_plus/ghost_report.py fa < report.json
```

برای Ghost Plus، آزمون C:

```sh
cc -std=c11 -Wall -Wextra -Werror -pedantic native/ghost_plus.c native/test_ghost_plus.c -o /tmp/ghost-plus-test
/tmp/ghost-plus-test
```

## محدودیت‌های مهم

هسته هنوز مسیر کامل ترافیک سیستم‌عامل را در Android/Linux/Windows، اتصال TUN/route/DNS سراسری و آزمون روی دستگاه واقعی برای همهٔ موتورها کامل نکرده است. انتخاب مسیر در سطح اتصال به معنی تجمیع بسته‌های چند تونل در یک جریان نیست. Ghost و نسخه‌های Plus تضمین دور زدن DPI نمی‌دهند و تنظیمات شبکه را خودکار تغییر نمی‌دهند.

## اسناد

- [Ghost Plus](GHOST_PLUS.md)
- [معماری](ARCHITECTURE.md)
- [نقشهٔ راه](ROADMAP.md)
- [یکپارچه‌سازی برنامهٔ میزبان](HOST_APP_INTEGRATION.md)
