package dev.pgcore

import org.junit.Assert.assertThrows
import org.junit.Test

class VpnServiceOptionsTest {
    @Test
    fun acceptsNumericIpv4AndIpv6DnsServers() {
        VpnServiceOptions(dnsServers = listOf("1.1.1.1", "2001:db8::1"))
    }

    @Test
    fun rejectsMalformedNumericAddresses() {
        assertThrows(IllegalArgumentException::class.java) {
            VpnServiceOptions(dnsServers = listOf("256.1.1.1"))
        }
        assertThrows(IllegalArgumentException::class.java) {
            VpnServiceOptions(dnsServers = listOf("::::"))
        }
        assertThrows(IllegalArgumentException::class.java) {
            VpnServiceOptions(dnsServers = listOf("2001:::1"))
        }
        assertThrows(IllegalArgumentException::class.java) {
            VpnServiceOptions(dnsServers = listOf("1:2:3:4:5:6:7:8:9"))
        }
    }

    @Test
    fun rejectsHostnamesAndDuplicateDnsServers() {
        assertThrows(IllegalArgumentException::class.java) {
            VpnServiceOptions(dnsServers = listOf("dns.example.com"))
        }
        assertThrows(IllegalArgumentException::class.java) {
            VpnServiceOptions(dnsServers = listOf("1.1.1.1", "1.1.1.1"))
        }
    }
}
