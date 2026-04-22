// I18n.cs - 舊 API 的薄封裝，轉呼叫 SetLang。
// 保留此型別是為了相容既有呼叫點（GameLink.I18n.I18n.Instance.T(...) / SetLocale / ChangeLanguage 等）。
// 實際的語料載入與語系狀態，都交給 SetLang（MonoBehaviour，自動在 Awake 讀 lan.csv）。
// 如果專案中沒人再用這個類別，之後可以直接刪掉。

using System.Collections.Generic;
using Cysharp.Threading.Tasks;
using UnityEngine;

namespace GameLink.I18n
{
    /// <summary>語系代碼常數（與 Cocos 一致）。</summary>
    public static class I18nConst
    {
        public const string DefaultLocale = "zh-Hant";
        public const string FallbackLocale = "zh-Hant";
        public const string CsvPath = "lan";
    }

    /// <summary>
    /// 舊 i18n 介面（相容用）。所有呼叫都會轉發到 SetLang；
    /// 由於 SetLang 會在場景 Awake 時自動載入 lan.csv，
    /// 這裡的 Init() / InitAsync() 不再需要手動呼叫（保留方法避免打破既有呼叫點）。
    /// </summary>
    public class I18n
    {
        public static I18n Instance { get; } = new I18n();

        /// <summary>是否已載入完成（= SetLang 是否就緒）。</summary>
        public bool IsReady => SetLang.Instance != null;

        /// <summary>目前語系代碼（zh-Hant / en-US）。</summary>
        public string CurrentLanguage =>
            SetLang.Instance != null
                ? SetLang.Instance.CurrentLanguageCode
                : I18nConst.DefaultLocale;

        /// <summary>已支援的語系代碼。</summary>
        public IReadOnlyList<string> Languages => new List<string>
        {
            SetLang.LanguageToCode(Language.ZhHant),
            SetLang.LanguageToCode(Language.EnUS),
        };

        /// <summary>
        /// 相容保留：SetLang 會自動載入 lan.csv，這裡不做事。
        /// </summary>
        public void Init()
        {
            // no-op：語料載入已經在 SetLang.Awake 完成。
        }

        /// <summary>相容保留的非同步版本：同樣為 no-op。</summary>
        public UniTask InitAsync()
        {
            return UniTask.CompletedTask;
        }

        /// <summary>取得 key 對應的翻譯。若 SetLang 尚未就緒會回傳 key。</summary>
        public string T(string key)
        {
            return SetLang.T(key);
        }

        /// <summary>切換語言（只在代碼屬於已支援語系時生效）。</summary>
        public void ChangeLanguage(string locale)
        {
            if (string.IsNullOrEmpty(locale) || SetLang.Instance == null) return;
            var lang = SetLang.CodeToLanguageStatic(locale);
            SetLang.Instance.SetLanguage(lang);
        }

        /// <summary>相容保留：內部轉呼叫 ChangeLanguage。</summary>
        public void SetLocale(string locale)
        {
            ChangeLanguage(locale);
        }
    }
}
