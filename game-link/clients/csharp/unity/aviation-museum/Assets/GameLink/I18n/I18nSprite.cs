// I18nSprite.cs - 依目前語系顯示對應 Sprite。
// 內部改走 SetLang，並訂閱 SetLang.OnLanguageChanged 自動刷新。

using UnityEngine;
using UnityEngine.UI;

namespace GameLink.I18n
{
    /// <summary>
    /// 依目前語系顯示對應 Sprite。
    /// Inspector：Locales 與 Sprites 兩陣列需一一對應（同 index 為同一語系），
    /// Locales 內填 "zh-Hant" / "en-US" 這類代碼；會比對 SetLang 目前的語系。
    /// </summary>
    public class I18nSprite : MonoBehaviour
    {
        [Tooltip("語系代碼，與 Sprites 依序對應（如 zh-Hant, en-US）")]
        public string[] locales = new string[0];

        [Tooltip("各語系對應的 Sprite，順序需與 Locales 一致")]
        public Sprite[] sprites = new Sprite[0];

        [Tooltip("要替換圖的 Image；留空則使用本節點上的 Image")]
        public Image imageTarget;

        private bool _subscribed;

        private void OnEnable()
        {
            TrySubscribe();
            Refresh();
        }

        private void Start()
        {
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

        /// <summary>依目前語系重新設定 Sprite。</summary>
        public void Refresh()
        {
            var target = imageTarget != null ? imageTarget : GetComponent<Image>();
            if (target == null)
            {
                Debug.LogWarning($"[I18nSprite] 節點 {name} 未設定 imageTarget 且本節點無 Image 組件");
                return;
            }
            int len = Mathf.Min(locales?.Length ?? 0, sprites?.Length ?? 0);
            if (len == 0)
            {
                Debug.LogWarning($"[I18nSprite] 節點 {name} 的 Locales 或 Sprites 為空");
                return;
            }

            string current = SetLang.Instance != null
                ? SetLang.Instance.CurrentLanguageCode
                : SetLang.LanguageToCode(Language.ZhHant);

            int index = 0;
            if (locales != null)
            {
                for (int i = 0; i < locales.Length; i++)
                {
                    if (locales[i] == current)
                    {
                        index = i;
                        break;
                    }
                }
            }
            if (sprites != null && index < sprites.Length && sprites[index] != null)
                target.sprite = sprites[index];
        }
    }
}
