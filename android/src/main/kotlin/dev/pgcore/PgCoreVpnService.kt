package dev.pgcore

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Intent
import android.net.VpnService
import android.os.Build
import android.os.ParcelFileDescriptor
import android.util.Log
import java.io.IOException
import java.net.InetAddress
import java.util.UUID
import java.util.concurrent.ExecutorService
import java.util.concurrent.Executors
import java.util.concurrent.Future
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicBoolean
import java.util.concurrent.atomic.AtomicLong

/**
 * Owns Android VPN lifecycle. The actual packet data plane is supplied by
 * PgCoreVpnBackend; without one, startup fails closed.
 * The host must call VpnService.prepare(context) and obtain consent first.
 */
class PgCoreVpnService : VpnService() {
    private val stopping = AtomicBoolean(false)
    private val generation = AtomicLong(0)
    private val lifecycleLock = Any()
    // Every ACTION_STOP caller must wait for the same cleanup attempt to finish.
    // Keep callbacks queued across failures so a later successful retry can finish them.
    private val pendingStopCallbacks = mutableListOf<() -> Unit>()
    private val executor: ExecutorService = Executors.newSingleThreadExecutor()
    @Volatile private var tun: ParcelFileDescriptor? = null
    @Volatile private var backend: PgCoreVpnBackend? = null
    @Volatile private var startTask: Future<*>? = null

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_STOP -> stopVpn { stopSelf(startId) }
            ACTION_START -> startVpn(intent)
            else -> stopSelf(startId)
        }
        return START_NOT_STICKY
    }

    private fun startVpn(intent: Intent?) {
        if (tun != null || startTask?.isDone == false) return
        stopping.set(false)
        val startGeneration = generation.incrementAndGet()
        createNotificationChannel()
        startForeground(NOTIFICATION_ID, notification("Preparing secure tunnel…"))
        startTask = executor.submit {
            var localTun: ParcelFileDescriptor? = null
            var localBackend: PgCoreVpnBackend? = null
            try {
                val options = VpnServiceOptions.fromIntent(intent)
                val config = VpnBackendConfig(
                    sessionId = UUID.randomUUID().toString(),
                    mtu = options.mtu,
                    dnsServers = options.dnsServers,
                )
                val builder = Builder().setSession("PG CORE").setMtu(config.mtu).setBlocking(true)
                options.allowedApplications.forEach { builder.addAllowedApplication(it) }
                options.disallowedApplications.forEach { builder.addDisallowedApplication(it) }
                if (config.enableIpv4) {
                    builder.addAddress("10.111.0.1", 32)
                    builder.addRoute("0.0.0.0", 0)
                }
                if (config.enableIpv6) {
                    builder.addAddress("fd42:7067:636f:7265::1", 128)
                    builder.addRoute("::", 0)
                }
                config.dnsServers.map { InetAddress.getByName(it) }.forEach(builder::addDnsServer)
                localTun = builder.establish() ?: throw IOException("Android denied VPN establishment")
                if (stopping.get() || generation.get() != startGeneration) throw IOException("VPN start cancelled")

                localBackend = PgCoreVpnBackendRegistry.create()
                val started = localBackend.start(localTun.fd, config) { socketFd -> protect(socketFd) }
                if (!started.ok) throw IOException(started.message ?: started.code)
                if (stopping.get() || generation.get() != startGeneration) throw IOException("VPN start cancelled")
                val healthy = localBackend.healthCheck()
                if (!healthy.ok) throw IOException(healthy.message ?: healthy.code)
                synchronized(lifecycleLock) {
                    if (stopping.get() || generation.get() != startGeneration) {
                        throw IOException("VPN start cancelled")
                    }
                    tun = localTun
                    backend = localBackend
                    localTun = null
                    localBackend = null
                }
                synchronized(lifecycleLock) {
                    if (!stopping.get() && generation.get() == startGeneration) {
                        updateNotification("PG CORE connected")
                    }
                }
            } catch (e: Exception) {
                Log.e(TAG, "VPN start failed; cleaning up", e)
                var backendCleanupFailed = false
                if (localBackend != null) {
                    try {
                        val result = localBackend.stop()
                        if (!result.ok) {
                            backendCleanupFailed = true
                            Log.e(TAG, "Backend cleanup failed: ${result.code}: ${result.message}")
                        }
                    } catch (stopError: Exception) {
                        backendCleanupFailed = true
                        Log.e(TAG, "Backend cleanup failed", stopError)
                    }
                }
                if (backendCleanupFailed && localBackend != null && localTun != null) {
                    synchronized(lifecycleLock) {
                        backend = localBackend
                        tun = localTun
                        stopping.set(false)
                    }
                    localBackend = null
                    localTun = null
                    updateNotification("Startup failed; cleanup incomplete. Tap Disconnect to retry.")
                    return@submit
                }
                try {
                    localTun?.close()
                } catch (closeError: IOException) {
                    Log.e(TAG, "TUN cleanup failed", closeError)
                    if (localTun != null) {
                        synchronized(lifecycleLock) {
                            tun = localTun
                            stopping.set(false)
                        }
                        localTun = null
                        updateNotification("TUN cleanup incomplete. Tap Disconnect to retry.")
                        return@submit
                    }
                }
                stopForegroundCompat()
                stopSelf()
            }
        }
    }

    /**
     * Stops the backend before closing its borrowed TUN descriptor. Failed
     * cleanup keeps ownership in this service and leaves Disconnect available
     * so the host can retry instead of reporting a false clean shutdown.
     */
    private fun stopVpn(onComplete: (() -> Unit)? = null) {
        synchronized(lifecycleLock) {
            onComplete?.let(pendingStopCallbacks::add)
            if (!stopping.compareAndSet(false, true)) return
            generation.incrementAndGet()
            startTask?.cancel(true)
        }
        executor.submit {
            // Read owned resources when this task actually runs. The cancelled
            // startup task may still need to publish resources for cleanup.
            val oldBackend: PgCoreVpnBackend?
            val oldTun: ParcelFileDescriptor?
            synchronized(lifecycleLock) {
                oldBackend = backend
                oldTun = tun
            }
            var backendStopped = true
            if (oldBackend != null) {
                try {
                    val result = oldBackend.stop()
                    if (!result.ok) {
                        backendStopped = false
                        Log.e(TAG, "Backend stop failed: ${result.code}: ${result.message}")
                    }
                } catch (e: Exception) {
                    backendStopped = false
                    Log.e(TAG, "Backend stop failed", e)
                }
            }
            if (!backendStopped) {
                stopping.set(false)
                updateNotification("Disconnect cleanup failed. Tap Disconnect to retry.")
                return@submit
            }

            try {
                oldTun?.close()
            } catch (e: IOException) {
                Log.e(TAG, "TUN close failed", e)
                stopping.set(false)
                updateNotification("TUN cleanup failed. Tap Disconnect to retry.")
                return@submit
            }

            val callbacks = synchronized(lifecycleLock) {
                if (backend === oldBackend) backend = null
                if (tun === oldTun) tun = null
                pendingStopCallbacks.toList().also { pendingStopCallbacks.clear() }
            }
            stopForegroundCompat()
            callbacks.forEach { callback ->
                try {
                    callback()
                } catch (e: Exception) {
                    Log.e(TAG, "VPN stop completion callback failed", e)
                }
            }
        }
    }

    override fun onRevoke() {
        stopVpn()
        super.onRevoke()
    }

    override fun onDestroy() {
        stopVpn()
        executor.shutdown()
        try {
            if (!executor.awaitTermination(STOP_TIMEOUT_SECONDS, TimeUnit.SECONDS)) executor.shutdownNow()
        } catch (e: InterruptedException) {
            executor.shutdownNow()
            Thread.currentThread().interrupt()
        }
        super.onDestroy()
    }

    private fun createNotificationChannel() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            (getSystemService(NOTIFICATION_SERVICE) as NotificationManager).createNotificationChannel(
                NotificationChannel(CHANNEL_ID, "PG CORE VPN", NotificationManager.IMPORTANCE_LOW)
            )
        }
    }

    private fun notification(text: String): Notification {
        val stopIntent = Intent(this, PgCoreVpnService::class.java).setAction(ACTION_STOP)
        val pending = PendingIntent.getService(
            this, 0, stopIntent,
            PendingIntent.FLAG_UPDATE_CURRENT or
                (if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.M) PendingIntent.FLAG_IMMUTABLE else 0)
        )
        val builder = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            Notification.Builder(this, CHANNEL_ID)
        } else {
            @Suppress("DEPRECATION") Notification.Builder(this)
        }
        return builder.setContentTitle("PG CORE VPN").setContentText(text)
            .setSmallIcon(android.R.drawable.stat_sys_warning)
            .setOngoing(tun != null || text != "Preparing secure tunnel…")
            .addAction(android.R.drawable.ic_menu_close_clear_cancel, "Disconnect", pending)
            .build()
    }

    private fun updateNotification(text: String) {
        (getSystemService(NOTIFICATION_SERVICE) as NotificationManager).notify(NOTIFICATION_ID, notification(text))
    }

    private fun stopForegroundCompat() {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.N) stopForeground(STOP_FOREGROUND_REMOVE)
        else {
            @Suppress("DEPRECATION")
            stopForeground(true)
        }
    }

    companion object {
        const val ACTION_START = "dev.pgcore.vpn.START"
        const val ACTION_STOP = "dev.pgcore.vpn.STOP"
        private const val TAG = "PgCoreVpnService"
        private const val CHANNEL_ID = "pg_core_vpn"
        private const val NOTIFICATION_ID = 7301
        private const val STOP_TIMEOUT_SECONDS = 3L
    }
}
