// AudioManager.cs — BGM 與音效（sound FX）全域播放器（單例）
// 使用方式：
//   1. 場景中放一空物件並掛上 AudioManager（或由程式呼叫 EnsureCreated()）。
//   2. 播放背景：AudioManager.Instance.PlayBgm(clip);
//   3. 播放音效：AudioManager.Instance.PlaySfx(clip);
using UnityEngine;

public class AudioManager : MonoBehaviour
{
    public static AudioManager Instance { get; private set; }

    [Header("若留空會自動在同一個 GameObject 上建立 AudioSource")]
    [SerializeField] private AudioSource bgmSource;
    [SerializeField] private AudioSource sfxSource;

    private float _bgmVolume = 1f;
    private float _sfxVolume = 1f;

    /// <summary>若場景已有實例則回傳；否則建立一個掛上 AudioManager 的物件。</summary>
    public static AudioManager EnsureCreated()
    {
        if (Instance != null)
            return Instance;

        var existing = FindFirstObjectByType<AudioManager>(FindObjectsInactive.Exclude);
        if (existing != null)
            return existing;

        var go = new GameObject("AudioManager");
        go.AddComponent<AudioManager>();
        return Instance;
    }

    private void Awake()
    {
        if (Instance != null && Instance != this)
        {
            Destroy(gameObject);
            return;
        }

        Instance = this;
        DontDestroyOnLoad(gameObject);
        EnsureAudioSources();
        ApplyVolumes();
    }

    private void OnDestroy()
    {
        if (Instance == this)
            Instance = null;
    }

    private void EnsureAudioSources()
    {
        if (bgmSource == null)
        {
            bgmSource = gameObject.AddComponent<AudioSource>();
            bgmSource.loop = true;
            bgmSource.playOnAwake = false;
        }
        else
        {
            bgmSource.loop = true;
            bgmSource.playOnAwake = false;
        }

        if (sfxSource == null)
        {
            sfxSource = gameObject.AddComponent<AudioSource>();
            sfxSource.loop = false;
            sfxSource.playOnAwake = false;
        }
        else
        {
            sfxSource.loop = false;
            sfxSource.playOnAwake = false;
        }
    }

    private void ApplyVolumes()
    {
        if (bgmSource != null)
            bgmSource.volume = _bgmVolume;
        if (sfxSource != null)
            sfxSource.volume = _sfxVolume;
    }

    /// <summary>目前是否正在播放 BGM。</summary>
    public bool IsBgmPlaying => bgmSource != null && bgmSource.isPlaying;

    /// <summary>目前 BGM 片段（可能為 null）。</summary>
    public AudioClip CurrentBgmClip => bgmSource != null ? bgmSource.clip : null;

    /// <summary>播放或切換背景音樂；若 clip 為 null 則停止。</summary>
    public void PlayBgm(AudioClip clip, float volume = 1f)
    {
        EnsureAudioSources();
        if (clip == null)
        {
            StopBgm();
            return;
        }

        _bgmVolume = Mathf.Clamp01(volume);
        bgmSource.volume = _bgmVolume;
        bgmSource.clip = clip;
        bgmSource.Play();
    }

    /// <summary>停止 BGM（保留 clip，若要清空請呼叫 StopBgm(clearClip: true)）。</summary>
    public void StopBgm(bool clearClip = false)
    {
        if (bgmSource == null) return;
        bgmSource.Stop();
        if (clearClip)
            bgmSource.clip = null;
    }

    /// <summary>暫停／繼續 BGM（不影響音效）。</summary>
    public void SetBgmPaused(bool paused)
    {
        if (bgmSource == null) return;
        if (paused)
            bgmSource.Pause();
        else
            bgmSource.UnPause();
    }

    /// <summary>播放一次性音效（可與上一個音效疊放）；volumeScale 為相對於音效總音量的倍率。</summary>
    public void PlaySfx(AudioClip clip, float volumeScale = 1f)
    {
        EnsureAudioSources();
        if (clip == null || sfxSource == null) return;
        sfxSource.PlayOneShot(clip, Mathf.Max(0f, volumeScale));
    }

    /// <summary>BGM 總音量（0～1）。</summary>
    public float BgmVolume
    {
        get => _bgmVolume;
        set
        {
            _bgmVolume = Mathf.Clamp01(value);
            if (bgmSource != null)
                bgmSource.volume = _bgmVolume;
        }
    }

    /// <summary>音效總音量（0～1）；PlaySfx 的 volumeScale 會再乘上此值。</summary>
    public float SfxVolume
    {
        get => _sfxVolume;
        set
        {
            _sfxVolume = Mathf.Clamp01(value);
            if (sfxSource != null)
                sfxSource.volume = _sfxVolume;
        }
    }
}
