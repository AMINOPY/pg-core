package dev.pgcore

import android.content.Context
import android.content.Intent
import android.net.VpnService
import android.os.Build

/**
 * Small host-app entry point for PG CORE's Android VPN service.
 *
 * The caller remains responsible for presenting the consent Intent returned by
 * [prepareIntent] and calling [start] only after Android grants permission.
 * This controller does not install a packet-forwarding backend.
 */
object PgCoreVpnController {
    /** Returns Android's VPN-consent activity Intent, or null if already authorized. */
    fun prepareIntent(context: Context): Intent? = VpnService.prepare(context)

    fun hasVpnPermission(context: Context): Boolean = VpnService.prepare(context) == null

    /**
     * Starts the foreground VPN service after permission is granted.
     *
     * Throws if permission has not been granted or Android refuses to start the
     * service. The service itself fails closed when no real backend is registered.
     */
    fun start(context: Context, options: VpnServiceOptions = VpnServiceOptions()) {
        check(hasVpnPermission(context)) {
            "VPN permission is required; launch prepareIntent(context) and wait for consent first"
        }

        val intent = Intent(context, PgCoreVpnService::class.java)
            .setAction(PgCoreVpnService.ACTION_START)
            .putExtra(VpnServiceOptions.EXTRA_MTU, options.mtu)
            .putStringArrayListExtra(
                VpnServiceOptions.EXTRA_DNS_SERVERS,
                ArrayList(options.dnsServers),
            )
            .putStringArrayListExtra(
                VpnServiceOptions.EXTRA_ALLOWED_APPLICATIONS,
                ArrayList(options.allowedApplications),
            )
            .putStringArrayListExtra(
                VpnServiceOptions.EXTRA_DISALLOWED_APPLICATIONS,
                ArrayList(options.disallowedApplications),
            )

        val component = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            context.startForegroundService(intent)
        } else {
            context.startService(intent)
        }
        checkNotNull(component) { "Android did not accept the PG CORE VPN service start request" }
    }

    /** Requests a normal service-owned stop so backend cleanup is attempted. */
    fun stop(context: Context) {
        val component = context.startService(
            Intent(context, PgCoreVpnService::class.java)
                .setAction(PgCoreVpnService.ACTION_STOP),
        )
        checkNotNull(component) {
            "Android did not accept the PG CORE VPN stop request; call stopService as a fallback"
        }
    }
}
