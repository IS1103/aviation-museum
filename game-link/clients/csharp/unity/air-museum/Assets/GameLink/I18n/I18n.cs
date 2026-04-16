// I18n.cs - 多國語言核心，對應 Cocos game-link-cocos-sdk/i18n/I18n.ts
// 譯文從 Resources/lan.csv 載入，格式與 Cocos 共用（第一列：,zh-Hant,en-US；之後每列：key, 繁中, 英文）

using System;
using System.Collections.Generic;
using System.Threading.Tasks;
using UnityEngine;

namespace GameLink.I18n
{
    /// <summary>語系代碼（與 Cocos 一致，如 zh-Hant, en-US）</summary>
    public static class I18nConst
    {
        public const string DefaultLocale = "zh-Hant";
        public const string FallbackLocale = "zh-Hant";
        /// <summary>CSV 在 Resources 下的路徑（不含副檔名）。請將 lan.csv 放在 Assets/Resources/ 下。</summary>
        public const string CsvPath = "lan";
    }

    /// <summary>
    /// 自訂 i18n，不依賴 Unity 內建 Localization。
    /// 譯文從 CSV 載入：Assets/Resources/lan.csv，格式與 Cocos 共用。
    /// </summary>
    public class I18n
    {
        private string _locale = I18nConst.DefaultLocale;
        private Dictionary<string, Dictionary<string, string>> _data = new Dictionary<string, Dictionary<string, string>>();
        private Task _initTask;

        public static I18n Instance { get; } = new I18n();

        /// <summary>是否已載入完成</summary>
        public bool IsReady => _data != null && _data.Count > 0;

        /// <summary>當前語言（如 zh-Hant, en-US）</summary>
        public string CurrentLanguage => _locale;

        /// <summary>已載入的語言列表</summary>
        public IReadOnlyList<string> Languages => _data != null ? new List<string>(_data.Keys) : new List<string>();

        /// <summary>
        /// 從 Resources 載入 CSV 並解析為譯文。可重複呼叫會重新載入。
        /// </summary>
        public void Init()
        {
            var ta = Resources.Load<TextAsset>(I18nConst.CsvPath);
            if (ta == null)
            {
                Debug.LogWarning("[I18n] 載入 CSV 失敗：Resources 中找不到 " + I18nConst.CsvPath);
                return;
            }
            _data = ParseCsvToData(ta.text);
        }

        /// <summary>非同步載入 CSV（供啟動流程 await）</summary>
        public Task InitAsync()
        {
            if (_initTask != null)
                return _initTask;
            _initTask = Task.Run(() =>
            {
                var ta = Resources.Load<TextAsset>(I18nConst.CsvPath);
                if (ta == null)
                {
                    Debug.LogWarning("[I18n] 載入 CSV 失敗：Resources 中找不到 " + I18nConst.CsvPath);
                    return;
                }
                _data = ParseCsvToData(ta.text);
            }).ContinueWith(_ => { _initTask = null; });
            return _initTask;
        }

        /// <summary>
        /// 取得 key 對應的翻譯；若當前語言沒有則用 fallback，再沒有則回傳 key。
        /// </summary>
        public string T(string key)
        {
            if (string.IsNullOrEmpty(key)) return key;
            if (_data != null && _data.TryGetValue(_locale, out var cur) && cur != null && cur.TryGetValue(key, out var v) && !string.IsNullOrEmpty(v))
                return v;
            if (_data != null && _data.TryGetValue(I18nConst.FallbackLocale, out var fallback) && fallback != null && fallback.TryGetValue(key, out var f) && !string.IsNullOrEmpty(f))
                return f;
            return key;
        }

        /// <summary>切換語言；之後 T() 會依新語言回傳。</summary>
        public void ChangeLanguage(string locale)
        {
            if (_data != null && _data.ContainsKey(locale))
                _locale = locale;
        }

        /// <summary>設定當前語言（不檢查是否存在，供初始化或覆寫用）。</summary>
        public void SetLocale(string locale)
        {
            if (!string.IsNullOrEmpty(locale))
                _locale = locale;
        }

        /// <summary>解析單行 CSV（支援雙引號包住的欄位內逗號）</summary>
        private static List<string> ParseCsvLine(string line)
        {
            var outList = new List<string>();
            var cur = "";
            var inQuotes = false;
            for (int i = 0; i < line.Length; i++)
            {
                var c = line[i];
                if (c == '"')
                    inQuotes = !inQuotes;
                else if ((c == ',' && !inQuotes) || (c == '\r' && !inQuotes))
                {
                    outList.Add(cur.Trim());
                    cur = "";
                    if (c == '\r') break;
                }
                else
                    cur += c;
            }
            outList.Add(cur.Trim());
            return outList;
        }

        /// <summary>
        /// 將 CSV 字串解析為 [locale -> [key -> string]]。
        /// 格式：第一列為標題，第一個欄位為空，其餘為語系代碼；之後每列第一欄為 key，其餘為對應語系文案。
        /// </summary>
        private static Dictionary<string, Dictionary<string, string>> ParseCsvToData(string csvText)
        {
            var data = new Dictionary<string, Dictionary<string, string>>();
            var lines = new List<string>();
            foreach (var ln in csvText.Split('\n'))
            {
                var t = ln.Trim();
                if (!string.IsNullOrEmpty(t)) lines.Add(t);
            }
            if (lines.Count == 0) return data;

            var header = ParseCsvLine(lines[0]);
            var locales = new List<string>();
            for (int i = 1; i < header.Count; i++)
            {
                var loc = header[i].Trim();
                if (!string.IsNullOrEmpty(loc))
                {
                    locales.Add(loc);
                    data[loc] = new Dictionary<string, string>();
                }
            }

            for (int r = 1; r < lines.Count; r++)
            {
                var row = ParseCsvLine(lines[r]);
                var key = row.Count > 0 ? row[0].Trim() : null;
                if (string.IsNullOrEmpty(key)) continue;
                for (int i = 0; i < locales.Count; i++)
                {
                    var locale = locales[i];
                    var value = (row.Count > i + 1 ? row[i + 1].Trim() : null) ?? "";
                    if (data.TryGetValue(locale, out var dict))
                        dict[key] = value;
                }
            }
            return data;
        }
    }
}
