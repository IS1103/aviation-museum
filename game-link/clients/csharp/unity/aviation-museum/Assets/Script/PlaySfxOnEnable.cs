// PlaySfxOnEnable.cs — GameObject 被 SetActive(true) / 啟用時播放音效（經 AudioManager）
using UnityEngine;

public class PlaySfxOnEnable : MonoBehaviour
{
    [Header("音效")]
    [SerializeField] private AudioClip clip;
    [Tooltip("傳給 AudioManager.PlaySfx 的倍率")]
    [SerializeField] private float volumeScale = 1f;

    [Header("行為")]
    [Tooltip("勾選後會跳過第一次 OnEnable（例如物件開場就為 active，只想在之後被打開時才播）")]
    [SerializeField] private bool skipFirstEnable;

    private bool _skippedOnce;

    private void OnEnable()
    {
        if (skipFirstEnable && !_skippedOnce)
        {
            _skippedOnce = true;
            return;
        }

        if (clip == null) return;

        var mgr = AudioManager.Instance ?? AudioManager.EnsureCreated();
        mgr.PlaySfx(clip, volumeScale);
    }
}
