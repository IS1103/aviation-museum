// BgmPlayer.cs — 場景內掛載用：在 Inspector 指定 BGM，交由 AudioManager 單例播放
using UnityEngine;

public class BgmPlayer : MonoBehaviour
{
    [Tooltip("要播放的背景音樂；可留空並改由程式呼叫 Play(AudioClip)。")]
    [SerializeField] private AudioClip bgmClip;

    [Range(0f, 1f)]
    [SerializeField] private float volume = 1f;

    [Tooltip("進入場景時是否自動播放 bgmClip。")]
    [SerializeField] private bool playOnStart = true;

    private void Start()
    {
        if (playOnStart)
            Play();
    }

    /// <summary>使用 Inspector 指定的 bgmClip 播放（clip 為 null 則不動作）。</summary>
    public void Play()
    {
        Play(bgmClip);
    }

    /// <summary>播放指定片段（會切換全域 BGM）。</summary>
    public void Play(AudioClip clip)
    {
        if (clip == null)
            return;
        AudioManager.EnsureCreated().PlayBgm(clip, volume);
    }

    /// <summary>停止目前 BGM；clearClip 為 true 時一併清除 AudioSource 上的 clip。</summary>
    public void Stop(bool clearClip = false)
    {
        var am = AudioManager.Instance;
        if (am != null)
            am.StopBgm(clearClip);
    }
}
