package com.github.kadaliao.kaixinguo.modules.player

import android.content.Context
import android.os.Handler
import android.os.Looper
import android.os.SystemClock
import android.view.TextureView
import android.widget.FrameLayout
import androidx.media3.common.C
import androidx.media3.common.MediaItem
import androidx.media3.common.PlaybackException
import androidx.media3.common.VideoSize as FrameSize
import androidx.media3.exoplayer.ExoPlayer
import dev.mygo.Native
import dev.mygo.NativeCloser
import dev.mygo.NativeUnsupportedException
import java.util.concurrent.CopyOnWriteArrayList
import kotlin.math.min
import kotlin.time.Duration
import kotlin.time.Duration.Companion.milliseconds

fun makePlayer(): Player {
    val ctx = Native.context ?: throw NativeUnsupportedException()
    return KxPlayer(ctx)
}

// KxPlayer plays a video with Media3 ExoPlayer on a TextureView. The app
// plays episodes of its local proxy, one at a time, so it reuses one
// player and Load replaces the video. Events reach Go on the UI thread,
// where the app's main loop runs too. The video fits the view as
// object-fit: contain does: the frame's transform letterboxes it.
private class KxPlayer(ctx: Context) : FrameLayout(ctx), Player {
    private val exo: ExoPlayer
    private val surface: TextureView
    private val events = CopyOnWriteArrayList<(State, String) -> Unit>()
    private val times = CopyOnWriteArrayList<(Duration, Duration) -> Unit>()
    private val handler = Handler(Looper.getMainLooper())
    // The stall check keeps its own handler: the ticker's clears all of
    // its messages, and the check must outlive that.
    private val stallHandler = Handler(Looper.getMainLooper())
    private var stallSince = 0L
    private var ticking = false

    init {
        // The app's player page ground (#1a1a1a), which the letterboxed
        // picture stands on, so no seam divides the bars around it.
        setBackgroundColor(0xFF1A1A1A.toInt())
        exo = try {
            ExoPlayer.Builder(ctx).build()
        } catch (t: Throwable) {
            throw NativeUnsupportedException()
        }
        surface = TextureView(ctx)
        addView(surface, LayoutParams(LayoutParams.MATCH_PARENT, LayoutParams.MATCH_PARENT))
        exo.setVideoTextureView(surface)
        exo.addListener(object : androidx.media3.common.Player.Listener {
            override fun onIsPlayingChanged(isPlaying: Boolean) {
                when {
                    isPlaying -> {
                        emit(State.STATE_PLAYING, "")
                        startTicker()
                    }
                    else -> {
                        // A pause the app made itself, a stall, or the end:
                        // the state change reports the end, so only settle
                        // the playhead here — a second ended would have the
                        // app skip the episode it just moved on to.
                        stopTicker()
                        tick()
                    }
                }
            }

            override fun onPlaybackStateChanged(state: Int) {
                when (state) {
                    androidx.media3.common.Player.STATE_BUFFERING -> {
                        emit(State.STATE_BUFFERING, "")
                        stopTicker()
                    }
                    androidx.media3.common.Player.STATE_ENDED -> {
                        emit(State.STATE_ENDED, "")
                        stopTicker()
                        tick()
                    }
                    androidx.media3.common.Player.STATE_READY -> if (exo.isPlaying) startTicker()
                    else -> {}
                }
            }

            override fun onVideoSizeChanged(size: FrameSize) =
                fit(size.width, size.height, size.unappliedRotationDegrees)

            override fun onPlayerError(error: PlaybackException) {
                stopTicker()
                emit(State.STATE_ERROR, reasonOf(error.errorCode))
            }
        })
        stallHandler.postDelayed({ checkStall() }, 1000)
    }

    // checkStall watches for a picture that stops coming. When the app
    // went to the background, or a stream died, ExoPlayer can sit in
    // BUFFERING for good, or play to a surface that went with the
    // background: eight seconds of that rebind the surface and prepare
    // the item again from the playhead, instead of spinning for ever.
    private fun checkStall() {
        val buffering = exo.playbackState == androidx.media3.common.Player.STATE_BUFFERING
        val noSurface = !surface.isAvailable
        if (exo.playWhenReady && (buffering || noSurface)) {
            if (stallSince == 0L) {
                stallSince = SystemClock.elapsedRealtime()
            } else if (SystemClock.elapsedRealtime() - stallSince >= 8_000) {
                stallSince = 0L
                recover()
            }
        } else {
            stallSince = 0L
        }
        stallHandler.postDelayed({ checkStall() }, 1000)
    }

    // recover puts the picture back on its surface and prepares the same
    // item again at the playhead: the surface may have gone with the
    // background, and a buffer that never fills starts over.
    private fun recover() {
        val item = exo.currentMediaItem ?: return
        val pos = exo.currentPosition.coerceAtLeast(0)
        exo.clearVideoSurface()
        exo.setVideoTextureView(surface)
        exo.setMediaItem(item, pos)
        exo.prepare()
        exo.play()
    }

    // fit sizes the TextureView to the frame's aspect, centered in the
    // player: the buffer stretches over the view, so a view of the frame's
    // aspect shows it whole, letterboxed by the player's black around it.
    // Rotation goes in first, for videos stored turned on their side.
    private fun fit(w: Int, h: Int, rotation: Int) {
        var fw = w.toFloat()
        var fh = h.toFloat()
        if (rotation == 90 || rotation == 270) {
            fw = h.toFloat()
            fh = w.toFloat()
        }
        val vw = width.toFloat()
        val vh = height.toFloat()
        if (vw <= 0f || vh <= 0f || fw <= 0f || fh <= 0f) return
        val scale = min(vw / fw, vh / fh)
        val lp = surface.layoutParams as LayoutParams
        val w2 = (fw * scale).toInt().coerceAtLeast(1)
        val h2 = (fh * scale).toInt().coerceAtLeast(1)
        if (lp.width == w2 && lp.height == h2) return
        lp.width = w2
        lp.height = h2
        lp.gravity = android.view.Gravity.CENTER
        surface.layoutParams = lp
    }

    override fun onSizeChanged(w: Int, h: Int, oldw: Int, oldh: Int) {
        super.onSizeChanged(w, h, oldw, oldh)
        val vs = exo.videoSize
        if (vs.width > 0 && vs.height > 0) fit(vs.width, vs.height, vs.unappliedRotationDegrees)
    }

    // Every layout pass checks the picture's size again: a turn of the
    // screen, or the full-screen player leaving it, must leave the frame
    // letterboxed in the new rect, never stretched to fill it.
    override fun onLayout(changed: Boolean, l: Int, t: Int, r: Int, b: Int) {
        super.onLayout(changed, l, t, r, b)
        val vs = exo.videoSize
        if (vs.width > 0 && vs.height > 0) fit(vs.width, vs.height, vs.unappliedRotationDegrees)
    }

    override fun load(url: String, start: Duration) {
        stopTicker()
        exo.setMediaItem(MediaItem.fromUri(url), start.inWholeMilliseconds.coerceAtLeast(0))
        exo.prepare()
    }

    override fun play() = exo.play()

    override fun pause() = exo.pause()

    override fun seek(to: Duration) {
        val d = exo.duration
        val ms = to.inWholeMilliseconds.coerceIn(0, if (d == C.TIME_UNSET) Long.MAX_VALUE else d)
        exo.seekTo(ms)
    }

    override fun setRate(rate: Double) = exo.setPlaybackSpeed(rate.toFloat())

    override fun videoSize(): VideoSize {
        val s = exo.videoSize
        return VideoSize(s.width.toLong(), s.height.toLong())
    }
    override fun onEvent(f: (State, String) -> Unit): AutoCloseable {
        events.add(f)
        return NativeCloser { events.remove(f) }
    }

    override fun onTime(f: (Duration, Duration) -> Unit): AutoCloseable {
        times.add(f)
        return NativeCloser { times.remove(f) }
    }

    override fun close() {
        events.clear()
        times.clear()
        stopTicker()
        stallHandler.removeCallbacksAndMessages(null)
        exo.release()
    }

    private fun emit(s: State, code: String) {
        for (f in events) f(s, code)
    }

    private fun tick() {
        val d = exo.duration
        val dur = if (d == C.TIME_UNSET) Duration.ZERO else d.milliseconds
        val pos = exo.currentPosition.milliseconds
        for (f in times) f(pos, dur)
    }

    // The playhead goes out at 250ms ticks while the video plays, so Go's
    // progress, history and 弹幕 follow it. Each tick asks Go for a whole
    // frame of its own, so the rate stays well under the render budget —
    // 66ms ones starved the UI thread and stuttered the playback.
    private fun startTicker() {
        if (ticking) return
        ticking = true
        loop()
    }

    private fun loop() {
        if (!ticking) return
        tick()
        handler.postDelayed({ loop() }, 250)
    }

    private fun stopTicker() {
        ticking = false
        handler.removeCallbacksAndMessages(null)
    }

    // reasonOf sorts a failure by the code's range: 2000s are input and
    // network, 3000s parsing, 4000s and 5000s decoding and rendering, and
    // 6000s DRM.
    private fun reasonOf(code: Int): String = when (code) {
        in 2000..2999 -> "io"
        in 6000..6999 -> "drm"
        else -> "decode"
    }
}
