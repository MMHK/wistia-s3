<template>
    <div ref="containerRef" class="relative min-h-[200px]">
        <div
            v-if="!playerReady"
            class="absolute inset-0 flex items-center justify-center bg-slate-900 rounded z-10"
        >
            <svg
                class="w-8 h-8 animate-spin text-white"
                fill="none"
                viewBox="0 0 24 24"
            >
                <circle
                    class="opacity-25"
                    cx="12"
                    cy="12"
                    r="10"
                    stroke="currentColor"
                    stroke-width="4"
                />
                <path
                    class="opacity-75"
                    fill="currentColor"
                    d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
                />
            </svg>
        </div>
        <div
            ref="embedRef"
            :class="embedClass"
            style="height: 100%; width: 100%"
        >
            &nbsp;
        </div>
    </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from "vue";

const props = defineProps({
    hashId: { type: String, required: true },
});

const emit = defineEmits(["timeupdate", "ready"]);

const containerRef = ref(null);
const embedRef = ref(null);
const playerReady = ref(false);
const videoEl = ref(null);
let scriptLoaded = false;
let scriptLoading = false;
let watcher = null;
let playerReadyHandler = null;

const embedClass = computed(() => {
    return `wistia_embed wistia_async_${props.hashId} videoFoam=true playsinline=true`;
});

const getVideoElement = () => {
    if (!containerRef.value) return null;
    return (
        containerRef.value.querySelector("video") ||
        containerRef.value.querySelector(".vjs-tech")
    );
};

const seekTo = (time) => {
    const video = videoEl.value || getVideoElement();
    if (video) {
        video.currentTime = time;
        if (video.paused) {
            video.play().catch(() => {});
        }
    }
};

const getCurrentTime = () => {
    const video = videoEl.value || getVideoElement();
    return video ? video.currentTime : 0;
};

const isPlaying = () => {
    const video = videoEl.value || getVideoElement();
    return video ? !video.paused : false;
};

const loadScript = () => {
    return new Promise((resolve, reject) => {
        if (scriptLoaded) {
            resolve();
            return;
        }
        if (scriptLoading) {
            const checkInterval = setInterval(() => {
                if (scriptLoaded) {
                    clearInterval(checkInterval);
                    resolve();
                }
            }, 100);
            return;
        }

        scriptLoading = true;
        const script = document.createElement("script");
        script.type = "text/javascript";
        script.src =
            "https://s3.ap-southeast-1.amazonaws.com/s3.test.mixmedia.com/wistia-backup/media/wistia-s3.min.js";
        script.onload = () => {
            scriptLoaded = true;
            scriptLoading = false;
            resolve();
        };
        script.onerror = () => {
            scriptLoading = false;
            reject(new Error("Failed to load player script"));
        };
        document.head.appendChild(script);
    });
};

const setupPlayerReadyListener = () => {
    playerReadyHandler = (e) => {
        const w = e.detail;
        if (w.getHashId() !== props.hashId) return;

        playerReady.value = true;

        watcher = w;

        w.bind("timeupdate", () => {
            const time = getCurrentTime();
            emit("timeupdate", time);
        });

        emit("ready");
    };

    window.addEventListener("video-player-ready", playerReadyHandler);
};

const waitForVideoElement = () => {
    return new Promise((resolve) => {
        let attempts = 0;
        const check = () => {
            const video = getVideoElement();
            if (video) {
                videoEl.value = video;
                resolve(video);
                return;
            }
            attempts++;
            if (attempts < 100) {
                setTimeout(check, 100);
            } else {
                resolve(null);
            }
        };
        check();
    });
};

onMounted(async () => {
    try {
        await loadScript();
        await nextTick();
        setupPlayerReadyListener();
        await waitForVideoElement();
    } catch (e) {
        console.error("Player init failed:", e);
    }
});

onUnmounted(() => {
    if (playerReadyHandler) {
        window.removeEventListener("video-player-ready", playerReadyHandler);
    }
});

watch(
    () => props.hashId,
    async () => {
        playerReady.value = false;
        videoEl.value = null;
        watcher = null;

        await nextTick();
        await waitForVideoElement();
    },
);

defineExpose({ seekTo, getCurrentTime, isPlaying });
</script>

<style scoped>
.wistia_embed {
    min-height: 200px;
}
</style>
