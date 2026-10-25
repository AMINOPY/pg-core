package dev.pgcore

import android.content.Intent

/**
 * Host-controlled Android VPN options. Application package filtering is
 * implemented by Android's VpnService.Builder; an empty filter means all apps.
 */
data class VpnServiceOptions(
    val mtu: Int = VpnBackendConfig.DEFAULT_MTU,
    val dnsServers: List<String> = emptyList(),
    val allowedApplications: List<String> = emptyList(),
    val disallowedApplications: List<String> = emptyList(),
) {
    init {
        require(mtu in VpnBackendConfig.MIN_MTU..VpnBackendConfig.MAX_MTU) { "mtu out of range" }
        require(dnsServers.size <= MAX_DNS_SERVERS) { "too many DNS servers" }
        require(allowedApplications.size <= MAX_APP_RULES) { "too many allowed applications" }
        require(disallowedApplications.size <= MAX_APP_RULES) { "too many disallowed applications" }
        require(allowedApplications.isEmpty() || disallowedApplications.isEmpty()) {
            "allowed and disallowed application lists cannot both be set"
        }
        require((allowedApplications + disallowedApplications).all(::validPackageName)) {
            "invalid Android application package name"
        }
        require(dnsServers.all(::looksLikeIpAddress)) { "DNS servers must be numeric IP addresses" }
        require(dnsServers.distinct().size == dnsServers.size) { "duplicate DNS server" }
    }

    companion object {
        const val EXTRA_MTU = "dev.pgcore.vpn.MTU"
        const val EXTRA_DNS_SERVERS = "dev.pgcore.vpn.DNS_SERVERS"
        const val EXTRA_ALLOWED_APPLICATIONS = "dev.pgcore.vpn.ALLOWED_APPLICATIONS"
        const val EXTRA_DISALLOWED_APPLICATIONS = "dev.pgcore.vpn.DISALLOWED_APPLICATIONS"
        const val MAX_DNS_SERVERS = 4
        const val MAX_APP_RULES = 256

        fun fromIntent(intent: Intent?): VpnServiceOptions = VpnServiceOptions(
            mtu = intent?.getIntExtra(EXTRA_MTU, VpnBackendConfig.DEFAULT_MTU)
                ?: VpnBackendConfig.DEFAULT_MTU,
            dnsServers = intent?.getStringArrayListExtra(EXTRA_DNS_SERVERS).orEmpty(),
            allowedApplications = intent?.getStringArrayListExtra(EXTRA_ALLOWED_APPLICATIONS).orEmpty(),
            disallowedApplications = intent?.getStringArrayListExtra(EXTRA_DISALLOWED_APPLICATIONS).orEmpty(),
        )

        private fun validPackageName(value: String): Boolean {
            if (value.isBlank() || value.length > 255) return false
            val parts = value.split('.')
            return parts.size >= 2 && parts.all { part ->
                part.isNotEmpty() && part.first().isLetter() &&
                    part.all { it.isLetterOrDigit() || it == '_' }
            }
        }

        private fun looksLikeIpAddress(value: String): Boolean {
            if (value.isBlank() || value.any { it.isWhitespace() }) return false
            return if (':' in value) {
                value.all { it.isDigit() || it.lowercaseChar() in 'a'..'f' || it == ':' || it == '.' }
            } else {
                val octets = value.split('.')
                octets.size == 4 && octets.all { octet ->
                    octet.isNotEmpty() && octet.all(Char::isDigit) &&
                        octet.toIntOrNull()?.let { it in 0..255 } == true
                }
            }
        }
    }
}
