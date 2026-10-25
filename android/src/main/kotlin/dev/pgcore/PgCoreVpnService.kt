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
import java.util.UUID
import java.util.concurrent.ExecutorService
import java.util.concurrent.Executors
import java.util.concurrent.Future
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicBoolean

/**
 * Owns Android VPN lifecycle. The actual packet data plane is supplied by
 * PgCoreVpnBackend; without one, startup fails closed.
 * The host must call VpnService.prepare(context) and obtain consent first.
 */
class PgCoreVpnService : VpnService() {
    private val stopping = AtomicBoolean(false)
    private val executor: ExecutorService = Executors.newSingleThreadExecutor()
    @Volatile private var tun: ParcelFileDescriptor? = null
    @Volatile private var backend: PgCoreVpnBackend? = null
    @Volatile private var startTask: Future<*>? = null

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_STOP -> { stopVpn(); stopSelf(startId) }
            ACTION_START -> startVpn()
            else -> stopSelf(startId)
        }
        return START_NOT_STICKY
    }

    private fun startVpn() {
        if (tun != null || startTask?.isDone == false) return
        stopping.set(false)
        createNotificationChannel()
        startForeground(NOTIFICATION_ID, notification("Preparing secure tunnel…"))
        startTask = executor.submit {
            var localTun: ParcelFileDescriptor? = null
            var localBackend: PgCoreVpnBackend? = null
            try {
                val config = VpnBackendConfig(sessionId = UUID.randomUUID().toString())
                val builder = Builder().setSession("PG CORE").setMtu(config.mtu).setBlocking(true)
                if (config.enableIpv4) {
                    builder.addAddress("10.111.0.1", 32)
                    builder.addRoute("0.0.0.0", 0)
                }
                if (config.enableIpv6) {
                    builder.addAddress("fd42:7067:636f:7265::1", 128)
                    builder.addRoute("::", 0)
                }
                config.dnsServers.forEach(builder::addDnsServer)
                localTun = builder.establish() ?: throw IOException("Android denied VPN establishment")
                if (stopping.get()) throw IOException("VPN start cancelled")

                localBackend = PgCoreVpnBackendRegistry.create()
                val started = localBackend.start(localTun.fd, config) { socketFd -> protect(socketFd) }
                if (!started.ok) throw IOException(started.message ?: started.code)
                val healthy = localBackend.healthCheck()
                if (!healthy.ok) throw IOException(healthy.message ?: healthy.code)
                if (stopping.get()) throw IOException("VPN start cancelled")

                tun = localTun
                backend = localBackend
                localTun = null
                localBackend = null
                updateNotification("PG CORE connected")
            } catch (e: Exception) {
                Log.e(TAG, "VPN start failed; cleaning up", e)
                try { localBackend?.stop() } catch (stopError: Exception) {
                    Log.e(TAG, "Backend cleanup failed", stopError)
                }
                try { localTun?.close() } catch (closeError: IOException) {
                    Log.e(TAG, "TUN cleanup failed", closeError)
                }
                stopForegroundCompat()
                stopSelf()
            }
        }
    }

    private fun stopVpn() {
        if (!stopping.compareAndSet(false, true)) return
        startTask?.cancel(true)
        val oldBackend = backend
        val oldTun = tun
        backend = null
        tun = null
        executor.submit {
            try { oldBackend?.stop() } catch (e: Exception) {
                Log.e(TAG, "Backend stop failed", e)
            } finally {
                try { oldTun?.close() } catch (e: IOException) {
                    Log.e(TAG, "TUN close failed", e)
                }
                stopForegroundCompat()
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
            getSystemService(NotificationManager::class.java).createNotificationChannel(
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
            .addAction(Notification.Action.Builder(null, "Disconnect", pending).build())
            .build()
    }

    private fun updateNotification(text: String) {
        getSystemService(NotificationManager::class.java).notify(NOTIFICATION_ID, notification(text))
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
