// TogglePlaySfx.cs — 掛在帶有 Toggle 的物件上，切換時播放音效（經 AudioManager）
using UnityEngine;
using UnityEngine.UI;

[RequireComponent(typeof(Toggle))]
public class TogglePlaySfx : MonoBehaviour
{
    public enum PlayMoment
    {
        /// <summary>每次 isOn 變更都播音（預設）</summary>
        EveryChange,
        /// <summary>僅在開啟時播音（適合同組 Toggle 選單）</summary>
        WhenTurnedOn,
        /// <summary>僅在關閉時播音</summary>
        WhenTurnedOff,
    }

    [Header("音效")]
    [SerializeField] private AudioClip clip;
    [Tooltip("傳給 AudioManager.PlaySfx 的倍率")]
    [SerializeField] private float volumeScale = 1f;

    [Header("行為")]
    [SerializeField] private PlayMoment playWhen = PlayMoment.EveryChange;
    [Tooltip("關閉時仍會觸發 Toggle，但不播音")]
    [SerializeField] private bool muteWhenInteractableFalse = true;

    private Toggle _toggle;

    private void Awake()
    {
        _toggle = GetComponent<Toggle>();
    }

    private void OnEnable()
    {
        if (_toggle != null)
            _toggle.onValueChanged.AddListener(OnToggleValueChanged);
    }

    private void OnDisable()
    {
        if (_toggle != null)
            _toggle.onValueChanged.RemoveListener(OnToggleValueChanged);
    }

    private void OnToggleValueChanged(bool isOn)
    {
        if (muteWhenInteractableFalse && _toggle != null && !_toggle.interactable)
            return;
        if (clip == null) return;

        switch (playWhen)
        {
            case PlayMoment.WhenTurnedOn:
                if (!isOn) return;
                break;
            case PlayMoment.WhenTurnedOff:
                if (isOn) return;
                break;
        }

        var mgr = AudioManager.Instance ?? AudioManager.EnsureCreated();
        mgr.PlaySfx(clip, volumeScale);
    }
}
