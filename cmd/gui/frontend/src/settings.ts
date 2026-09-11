import { FeedService } from "../bindings/wisp/cmd/gui";
import { Settings } from "../bindings/wisp/internal/api";
import { requireEl } from "./dom";
import { audioEl } from "./player";

const form = requireEl<HTMLFormElement>("settings-form");
const themeEl = requireEl<HTMLSelectElement>("settings-theme");
const speedEl = requireEl<HTMLSelectElement>("settings-speed");
const whisperEl = requireEl<HTMLSelectElement>("settings-whisper");
const cloudEl = requireEl<HTMLInputElement>("settings-cloud");

function apply(s: Settings): void {
    document.documentElement.dataset.theme = s.theme;
    audioEl.playbackRate = s.playbackSpeed;
    themeEl.value = s.theme;
    speedEl.value = String(s.playbackSpeed);
    whisperEl.value = s.whisperModel;
    cloudEl.checked = s.cloudTranscription;
}

function readForm(): Settings {
    return new Settings({
        theme: themeEl.value,
        playbackSpeed: Number(speedEl.value),
        whisperModel: whisperEl.value,
        cloudTranscription: cloudEl.checked,
    });
}

form.addEventListener("change", async () => {
    try {
        apply(await FeedService.SaveSettings(readForm()));
    } catch { /* next load restores last saved */ }
});

FeedService.GetSettings().then(apply).catch(() => apply(new Settings({
    theme: "dark",
    playbackSpeed: 1,
    whisperModel: "base",
})));
