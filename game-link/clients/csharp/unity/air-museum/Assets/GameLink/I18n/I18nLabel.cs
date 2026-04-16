// I18nLabel.cs - 綁定 i18n key 到 UI 文字，對應 Cocos I18nLabel.ts

using UnityEngine;
using UnityEngine.UI;

namespace GameLink.I18n
{
    /// <summary>
    /// 綁定 i18n key 到 Label：會在 Start 時把 Text 的 text 設為對應翻譯。
    /// 若未指定 textTarget，會使用同一節點上的 Text 組件。
    /// 切換語系後可呼叫 Refresh() 更新顯示。
    /// </summary>
    public class I18nLabel : MonoBehaviour
    {
        [Tooltip("i18n 鍵值，例如 loading.btn_confirm")]
        public string key = "";

        [Tooltip("要顯示翻譯的 Text；留空則使用本節點上的 Text")]
        public Text textTarget;

        private void Start()
        {
            Refresh();
        }

        /// <summary>依目前語系重新設定文字。切換語系後可手動呼叫以更新顯示。</summary>
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
            target.text = I18n.Instance.T(key);
        }
    }
}
