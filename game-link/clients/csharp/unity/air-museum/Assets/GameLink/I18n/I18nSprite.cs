// I18nSprite.cs - 依目前語系顯示對應 Sprite，對應 Cocos I18nSprite.ts

using UnityEngine;
using UnityEngine.UI;

namespace GameLink.I18n
{
    /// <summary>
    /// 依目前語系顯示對應 Sprite。
    /// Inspector：Locales 與 Sprites 兩陣列需一一對應（同 index 為同一語系），在 Sprites 裡直接拖入各語系的圖即可。
    /// 切換語系後可呼叫 Refresh() 更新顯示。
    /// </summary>
    public class I18nSprite : MonoBehaviour
    {
        [Tooltip("語系代碼，與 Sprites 依序對應（如 zh-Hant, en-US）")]
        public string[] locales = new string[0];

        [Tooltip("各語系對應的 Sprite，順序需與 Locales 一致")]
        public Sprite[] sprites = new Sprite[0];

        [Tooltip("要替換圖的 Image；留空則使用本節點上的 Image")]
        public Image imageTarget;

        private void Start()
        {
            Refresh();
        }

        /// <summary>依目前語系重新設定 Sprite。切換語系後可手動呼叫以更新顯示。</summary>
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
            string current = I18n.Instance.CurrentLanguage;
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
