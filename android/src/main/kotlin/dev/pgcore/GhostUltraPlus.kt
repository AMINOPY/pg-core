package dev.pgcore

/** Local interpretation of collected observations; no probing or network mutation. */
enum class GhostLanguage { ENGLISH, PERSIAN }
data class GhostObservation(val samples: Int, val dnsSuccesses: Int, val httpsSuccesses: Int, val tlsErrors: Int = 0, val resets: Int = 0, val timeouts: Int = 0, val http403: Int = 0, val http451: Int = 0)
data class GhostUltraPlusReport(val status: String, val confidencePercent: Int, val title: String, val details: String, val recommendation: String)
object GhostUltraPlus {
    fun analyze(o: GhostObservation, language: GhostLanguage = GhostLanguage.ENGLISH): GhostUltraPlusReport {
        require(o.samples in 1..20) { "samples must be in 1..20" }
        val counters = listOf(o.dnsSuccesses, o.httpsSuccesses, o.tlsErrors, o.resets, o.timeouts, o.http403, o.http451)
        require(counters.all { it in 0..o.samples }) { "observation counters exceed sample count" }
        val fa = language == GhostLanguage.PERSIAN
        return when {
            o.dnsSuccesses == o.samples && o.httpsSuccesses == o.samples -> GhostUltraPlusReport("healthy",90,if (fa) "اتصال موفق بود" else "Connectivity succeeded",if (fa) "DNS و HTTPS در همهٔ نمونه‌ها موفق بودند." else "DNS and HTTPS succeeded in every sample.",if (fa) "فعلاً تغییری لازم نیست." else "No change is indicated.")
            o.dnsSuccesses == 0 -> GhostUltraPlusReport("dns_failure",75,if (fa) "خطای DNS" else "DNS failure",if (fa) "علت مشخص نیست و این نتیجه اثبات DPI نیست." else "Cause is unknown; this does not prove DPI.",if (fa) "نام میزبان، DNS و صفحه ورود شبکه را بررسی کنید." else "Check hostname, resolver, and captive portal.")
            o.http403 + o.http451 > 0 -> GhostUltraPlusReport("possible_filtering",55,if (fa) "پاسخ منع دسترسی" else "Policy-denial response",if (fa) "این پاسخ به‌تنهایی وجود DPI را اثبات نمی‌کند." else "This response alone does not prove DPI.",if (fa) "سیاست سرور و وضعیت مقصد را بررسی کنید." else "Verify server policy and endpoint status.")
            else -> GhostUltraPlusReport("inconclusive",35,if (fa) "نتیجه نامشخص" else "Inconclusive result",if (fa) "خطای شبکه علت‌های گوناگونی دارد." else "Network errors can have multiple causes.",if (fa) "زمان دستگاه، گواهی TLS و سلامت سرور را بررسی کنید." else "Check device time, TLS certificates, and server health.")
        }
    }
}
