// I18nLabel.cs - 綁定 i18n key 到 UI 文字。
// 內部改走 SetLang（自動載入 lan.csv，不需手動 Init），並訂閱 SetLang.OnLanguageChanged 自動刷新。

using UnityEngine;
using UnityEngine.UI;

namespace GameLink.I18n
{
    /// <summary>
    /// 綁定 i18n key 到 Label：啟用時會把 Text 的 text 設為對應翻譯，
    /// 並訂閱 SetLang.OnLanguageChanged，切換語系時自動刷新；停用時取消訂閱。
    /// 若未指定 textTarget，會使用同一節點上的 Text 組件。
    /// </summary>
    public class I18nLabel : MonoBehaviour
    {
        [Tooltip("i18n 鍵值，例如 loading.btn_confirm")]
        public string key = "";

        [Tooltip("要顯示翻譯的 Text；留空則使用本節點上的 Text")]
        public Text textTarget;

        private bool _subscribed;

        private void OnEnable()
        {
            TrySubscribe();
            Refresh();
        }

        private void Start()
        {
            // SetLang.Instance 在自己 Awake 裡指派，這裡再試一次以確保訂閱到
            TrySubscribe();
            Refresh();
        }

        private void OnDisable()
        {
            Unsubscribe();
        }

        private void OnDestroy()
        {
            Unsubscribe();
        }

        private void TrySubscribe()
        {
            if (_subscribed) return;
            if (SetLang.Instance == null) return;
            SetLang.Instance.OnLanguageChanged += OnLanguageChanged;
            _subscribed = true;
        }

        private void Unsubscribe()
        {
            if (!_subscribed) return;
            if (SetLang.Instance != null)
            {
                SetLang.Instance.OnLanguageChanged -= OnLanguageChanged;
            }
            _subscribed = false;
        }

        private void OnLanguageChanged(Language _)
        {
            Refresh();
        }

        /// <summary>依目前語系重新設定文字。</summary>
        public void Refresh()
        {
            var target = textTarget != null ? textTarget : GetComponent<Text>();
            if (target == null)
            {
                Debug.LogWarning($"[I18nLabel] 節點 {name} 未設定 textTarget 且本節點無 Text 組件");
                return;
            }
            if (string.IsNullOrEmpty(key))
            {
                Debug.LogWarning($"[I18nLabel] 節點 {name} 未設定 key");
                return;
            }
            target.text = SetLang.T(key);
        }
    }
}
