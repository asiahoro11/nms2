(function (global) {
    const sessions = new Map();
    const DEFAULT_TIMEOUT_MS = 20000;
    const WEBRTC_FIRST_TIMEOUT_MS = 3500;
    const MSE_START_TIMEOUT_MS = 8000;
    const MSE_FIRST_MEDIA_TIMEOUT_MS = 12000;
    const MSE_LATENCY_TUNE_MS = 350;
    const DEFAULT_MSE_CODECS = [
        'avc1.640029',
        'avc1.64002A',
        'avc1.640033',
        'hvc1.1.6.L153.B0'
    ];
    const MAX_MSE_QUEUE = 1;

    function supportsWebRTC() {
        return typeof global.RTCPeerConnection === 'function' && typeof global.WebSocket === 'function';
    }

    function mediaSourceClass() {
        return global.ManagedMediaSource || global.MediaSource || global.WebKitMediaSource || null;
    }

    function supportsMSE() {
        const MediaSourceImpl = mediaSourceClass();
        return typeof global.WebSocket === 'function'
            && !!MediaSourceImpl
            && (global.ManagedMediaSource || (global.URL && typeof global.URL.createObjectURL === 'function'));
    }

    function supports() {
        return supportsWebRTC() || supportsMSE();
    }

    function authToken() {
        if (typeof global.getAuthToken === 'function') return global.getAuthToken() || '';
        try { return localStorage.getItem('nms-token') || localStorage.getItem('token') || ''; } catch (_) { return ''; }
    }

    function wsUrl(camId, options = {}) {
        const scheme = global.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const params = new URLSearchParams();
        const token = authToken();
        if (token) params.set('token', token);
        if (options.profile) params.set('profile', options.profile);
        const query = params.toString();
        return `${scheme}//${global.location.host}/api/v1/cameras/${encodeURIComponent(camId)}/stream/webrtc/ws${query ? `?${query}` : ''}`;
    }

    function isTypeSupported(MediaSourceImpl, mime) {
        if (!MediaSourceImpl || typeof MediaSourceImpl.isTypeSupported !== 'function') return true;
        try { return MediaSourceImpl.isTypeSupported(mime); } catch (_) { return false; }
    }

    function supportedMSECodecs(MediaSourceImpl) {
        return DEFAULT_MSE_CODECS
            .filter(codec => isTypeSupported(MediaSourceImpl, `video/mp4; codecs="${codec}"`))
            .join();
    }

    function prepareVideo(video, camId, mode) {
        const id = video.id || `camera-preview-${camId}-${Date.now()}`;
        stop(id);
        video.id = id;
        video.autoplay = true;
        video.muted = true;
        video.playsInline = true;
        video.dataset.previewMode = mode;
        video.dataset.camId = String(camId);
        video.dataset.streamStartedAt = '';
        return id;
    }

    function stop(target) {
        const id = typeof target === 'string' ? target : target?.id;
        if (!id) return;
        const session = sessions.get(id);
        if (!session) return;
        sessions.delete(id);
        clearTimeout(session.timer);
        clearInterval(session.latencyTimer);
        try { session.ws && session.ws.close(); } catch (_) {}
        try { session.pc && session.pc.close(); } catch (_) {}
        try {
            if (session.video && session.video.srcObject) {
                if (typeof session.video.srcObject.getTracks === 'function') {
                    session.video.srcObject.getTracks().forEach(track => track.stop());
                }
                session.video.srcObject = null;
            }
        } catch (_) {}
        try {
            if (session.video && session.objectUrl && session.video.src === session.objectUrl) {
                session.video.removeAttribute('src');
                session.video.load();
            }
        } catch (_) {}
        try {
            if (session.objectUrl) global.URL.revokeObjectURL(session.objectUrl);
        } catch (_) {}
    }

    function fail(id, reason) {
        const session = sessions.get(id);
        if (!session) return;
        const onError = session.onError;
        const video = session.video;
        stop(id);
        if (typeof onError === 'function') onError(reason, video);
    }

    function resetTimer(session, reason, timeoutMs) {
        clearTimeout(session.timer);
        session.timer = setTimeout(() => fail(session.id, reason), timeoutMs);
    }

    async function start(video, camId, options = {}) {
        if (!video || !camId || !supportsWebRTC()) {
            throw new Error('webrtc_not_supported');
        }

        const id = prepareVideo(video, camId, 'webrtc');
        const pc = new RTCPeerConnection({ bundlePolicy: 'max-bundle' });
        const ws = new WebSocket(wsUrl(camId, options));
        video.srcObject = null;

        const session = {
            id,
            pc,
            ws,
            video,
            connected: false,
            onError: options.onError,
            timer: setTimeout(() => fail(id, 'webrtc_timeout'), options.timeoutMs || DEFAULT_TIMEOUT_MS)
        };
        sessions.set(id, session);

        pc.ontrack = (event) => {
            const stream = event.streams && event.streams[0] ? event.streams[0] : new MediaStream([event.track]);
            video.srcObject = stream;
            session.connected = true;
            clearTimeout(session.timer);
            video.dataset.streamStartedAt = String(Date.now());
            video.play().catch(() => {});
            if (typeof options.onReady === 'function') options.onReady(video);
        };

        pc.onconnectionstatechange = () => {
            if (['failed', 'closed'].includes(pc.connectionState)) {
                fail(id, pc.connectionState);
            }
        };

        pc.onicecandidate = (event) => {
            if (ws.readyState !== WebSocket.OPEN) return;
            ws.send(JSON.stringify({
                type: 'webrtc/candidate',
                value: event.candidate ? event.candidate.candidate : ''
            }));
        };

        ws.onopen = async () => {
            try {
                pc.addTransceiver('video', { direction: 'recvonly' });
                const offer = await pc.createOffer();
                await pc.setLocalDescription(offer);
                ws.send(JSON.stringify({ type: 'webrtc/offer', value: offer.sdp }));
            } catch (error) {
                fail(id, error.message || 'offer_failed');
            }
        };

        ws.onmessage = async (event) => {
            try {
                const message = JSON.parse(event.data);
                if (message.type === 'webrtc/answer') {
                    await pc.setRemoteDescription({ type: 'answer', sdp: message.value });
                } else if (message.type === 'webrtc/candidate' && message.value) {
                    await pc.addIceCandidate({ candidate: message.value });
                } else if (message.type === 'error') {
                    fail(id, message.value || 'signal_error');
                }
            } catch (error) {
                fail(id, error.message || 'signal_failed');
            }
        };

        ws.onerror = () => fail(id, 'websocket_error');
        ws.onclose = () => {
            const current = sessions.get(id);
            if (current && !current.connected) fail(id, 'websocket_closed');
        };
    }

    function markMSEReady(session, options) {
        if (session.connected) return;
        session.connected = true;
        clearTimeout(session.timer);
        session.video.dataset.streamStartedAt = String(Date.now());
        session.video.play().catch(() => {});
        if (typeof options.onReady === 'function') options.onReady(session.video);
    }

    function tuneMSELatency(session) {
        const video = session.video;
        if (!video || !video.buffered || video.buffered.length === 0) return;
        try {
            const end = video.buffered.end(video.buffered.length - 1);
            const start = video.buffered.start(0);
            const lag = end - video.currentTime;
            if (!Number.isFinite(lag)) return;
            if (lag > 0.45) {
                video.currentTime = Math.max(0, end - 0.08);
                video.playbackRate = 1;
            } else if (lag > 0.2) {
                video.playbackRate = 1.08;
            } else {
                video.playbackRate = 1;
            }
            if (session.sourceBuffer && !session.sourceBuffer.updating && session.mediaSource.readyState === 'open' && end - start > 1.2) {
                const removeEnd = Math.max(start, end - 0.8);
                if (removeEnd > start + 0.1) session.sourceBuffer.remove(start, removeEnd);
            }
        } catch (_) {}
    }

    function appendMSEChunk(session, data, options) {
        if (!data || !session.sourceBuffer || session.sourceBuffer.updating || session.mediaSource.readyState !== 'open') {
            if (data) session.queue = [data].slice(-MAX_MSE_QUEUE);
            return;
        }
        try {
            session.sourceBuffer.appendBuffer(data);
            markMSEReady(session, options);
        } catch (error) {
            fail(session.id, error.message || 'mse_append_failed');
        }
    }

    function flushMSEQueue(session, options) {
        if (!session.sourceBuffer || session.sourceBuffer.updating || session.queue.length === 0) return;
        const next = session.queue.shift();
        appendMSEChunk(session, next, options);
    }

    function handleMSEControl(session, raw, options) {
        let message;
        try {
            message = JSON.parse(raw);
        } catch (_) {
            return;
        }
        if (message.type === 'error') {
            fail(session.id, message.value || 'mse_error');
            return;
        }
        if (message.type !== 'mse' || session.sourceBuffer) return;

        const value = String(message.value || '').trim();
        if (!value) {
            fail(session.id, 'mse_missing_codecs');
            return;
        }
        const mime = value.startsWith('video/') ? value : `video/mp4; codecs="${value.replace(/"/g, '')}"`;
        const MediaSourceImpl = mediaSourceClass();
        if (!isTypeSupported(MediaSourceImpl, mime)) {
            fail(session.id, 'mse_codec_unsupported');
            return;
        }

        try {
            const sourceBuffer = session.mediaSource.addSourceBuffer(mime);
            sourceBuffer.mode = 'segments';
            sourceBuffer.addEventListener('updateend', () => {
                tuneMSELatency(session);
                flushMSEQueue(session, options);
            });
            sourceBuffer.addEventListener('error', () => fail(session.id, 'mse_sourcebuffer_error'));
            session.sourceBuffer = sourceBuffer;
            session.latencyTimer = setInterval(() => tuneMSELatency(session), MSE_LATENCY_TUNE_MS);
            resetTimer(session, 'mse_media_timeout', options.mediaTimeoutMs || MSE_FIRST_MEDIA_TIMEOUT_MS);
            flushMSEQueue(session, options);
        } catch (error) {
            fail(session.id, error.message || 'mse_sourcebuffer_failed');
        }
    }

    async function startMSE(video, camId, options = {}) {
        if (!video || !camId || !supportsMSE()) {
            throw new Error('mse_not_supported');
        }

        const MediaSourceImpl = mediaSourceClass();
        const id = prepareVideo(video, camId, 'mse');
        const mediaSource = new MediaSourceImpl();
        const ws = new WebSocket(wsUrl(camId, options));
        ws.binaryType = 'arraybuffer';

        video.srcObject = null;
        let objectUrl = '';
        if (global.ManagedMediaSource && mediaSource instanceof global.ManagedMediaSource) {
            video.disableRemotePlayback = true;
            video.srcObject = mediaSource;
        } else {
            objectUrl = global.URL.createObjectURL(mediaSource);
            video.src = objectUrl;
        }

        const session = {
            id,
            ws,
            video,
            mediaSource,
            objectUrl,
            sourceBuffer: null,
            queue: [],
            connected: false,
            onError: options.onError,
            timer: setTimeout(() => fail(id, 'mse_timeout'), options.timeoutMs || DEFAULT_TIMEOUT_MS)
        };
        sessions.set(id, session);

        const requestMSE = () => {
            const current = sessions.get(id);
            if (!current || current.requestedMSE) return;
            if (ws.readyState !== WebSocket.OPEN || mediaSource.readyState !== 'open') return;
            current.requestedMSE = true;
            ws.send(JSON.stringify({ type: 'mse', value: options.codecs || supportedMSECodecs(MediaSourceImpl) }));
        };

        mediaSource.addEventListener('sourceopen', requestMSE, { once: true });
        mediaSource.addEventListener('sourceended', () => fail(id, 'mse_source_ended'));
        mediaSource.addEventListener('error', () => fail(id, 'mse_mediasource_error'));

        ws.onopen = requestMSE;
        ws.onmessage = async (event) => {
            const current = sessions.get(id);
            if (!current) return;
            if (typeof event.data === 'string') {
                handleMSEControl(current, event.data, options);
                return;
            }
            try {
                const data = event.data instanceof Blob ? await event.data.arrayBuffer() : event.data;
                appendMSEChunk(current, data, options);
            } catch (error) {
                fail(id, error.message || 'mse_chunk_failed');
            }
        };
        ws.onerror = () => fail(id, 'websocket_error');
        ws.onclose = () => {
            const current = sessions.get(id);
            if (current && !current.connected) fail(id, 'websocket_closed');
        };
    }

    function runAttempt(attempt, video, camId, options) {
        return new Promise((resolve, reject) => {
            let ready = false;
            const merged = Object.assign({}, options, {
                profile: attempt.profile,
                timeoutMs: attempt.timeoutMs || options.timeoutMs,
                onReady: (el) => {
                    ready = true;
                    if (el) {
                        el.dataset.previewTransport = attempt.kind;
                        el.dataset.previewProfile = attempt.profile || 'direct';
                    }
                    if (typeof options.onReady === 'function') options.onReady(el, attempt);
                    resolve(el);
                },
                onError: (reason, el) => {
                    if (!ready) {
                        reject({ reason, video: el || video });
                        return;
                    }
                    if (typeof options.onError === 'function') options.onError(reason, el || video, attempt);
                }
            });

            try {
                const starter = attempt.kind === 'mse' ? startMSE : start;
                Promise.resolve(starter(video, camId, merged)).catch((error) => {
                    reject({ reason: error.message || String(error), video });
                });
            } catch (error) {
                reject({ reason: error.message || String(error), video });
            }
        });
    }

    async function startLowLatency(video, camId, options = {}) {
        const attempts = [];
        if (supportsWebRTC()) {
            attempts.push({ kind: 'webrtc', profile: '', timeoutMs: options.webrtcTimeoutMs || WEBRTC_FIRST_TIMEOUT_MS });
        }
        if (supportsMSE()) {
            attempts.push({ kind: 'mse', profile: '', timeoutMs: options.mseTimeoutMs || MSE_START_TIMEOUT_MS });
        }
        if (attempts.length === 0) {
            throw new Error('preview_not_supported');
        }

        let target = video;
        let lastReason = 'preview_failed';
        for (const attempt of attempts) {
            if (!target || !target.isConnected) break;
            try {
                return await runAttempt(attempt, target, camId, options);
            } catch (failure) {
                lastReason = failure.reason || lastReason;
                target = failure.video || target;
            }
        }
        if (typeof options.onError === 'function') options.onError(lastReason, target);
        throw new Error(lastReason);
    }

    function stopAll(root = document) {
        if (!root || typeof root.querySelectorAll !== 'function') return;
        root.querySelectorAll('video[data-preview-mode="webrtc"],video[data-preview-mode="mse"],video[data-webrtc="1"]').forEach(stop);
    }

    global.CameraWebRTC = { supports, supportsWebRTC, supportsMSE, start, startMSE, startLowLatency, stop, stopAll };
})(window);
