// ButtonPlaySfx.cs — 掛在帶有 Button 的物件上，點擊時播放音效（經 AudioManager）
using UnityEngine;
using UnityEngine.UI;

[RequireComponent(typeof(Button))]
public class ButtonPlaySfx : MonoBehaviour
{
    [Header("音效")]
    [SerializeField] private AudioClip clip;
    [Tooltip("傳給 AudioManager.PlaySfx 的倍率")]
    [SerializeField] private float volumeScale = 1f;

    [Header("行為")]
    [Tooltip("關閉時仍會觸發 Button.onClick，但不播音")]
    [SerializeField] private bool muteWhenInteractableFalse = true;

    private Button _button;

    private void Awake()
    {
        _button = GetComponent<Button>();
    }

    private void OnEnable()
    {
        if (_button != null)
            _button.onClick.AddListener(OnButtonClicked);
    }

    private void OnDisable()
    {
        if (_button != null)
            _button.onClick.RemoveListener(OnButtonClicked);
    }

    private void OnButtonClicked()
    {
        if (muteWhenInteractableFalse && _button != null && !_button.interactable)
            return;
        if (clip == null) return;

        var mgr = AudioManager.Instance ?? AudioManager.EnsureCreated();
        mgr.PlaySfx(clip, volumeScale);
    }
}
