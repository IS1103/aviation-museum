using System;
using System.Globalization;
using UnityEngine;
using UnityEngine.UI;

/// <summary>
/// 個人資料／任務狀態：從 PlayerPrefs 與本地時間更新 UI，並於 OnEnable 輸出除錯資訊。
/// </summary>
public class DisplayPlayerInfo : MonoBehaviour
{
    public const string PrefGameScore = "air_museum_game_score";
    public const string PrefLandingScore = "air_museum_landing_score";
    public const string PrefRanking = "air_museum_ranking";
    public const string PrefMissionFlight = "air_museum_mission_flight";
    public const string PrefMissionFighterMod = "air_museum_mission_fighter_mod";
    public const string PrefMissionStory = "air_museum_mission_story";

    private static readonly CultureInfo ScoreCulture = CultureInfo.InvariantCulture;

    [Header("個人資料（Inspector 可選指派）")]
    [SerializeField] private Text playerIdText;
    [SerializeField] private Text pilotNameText;
    [SerializeField] private Text gameScoreText;
    [SerializeField] private Text landingScoreText;
    [SerializeField] private Text rankingText;
    [SerializeField] private Text certificationDateText;

    [SerializeField] private Color sDoneColor, sFailedColor;

    // [Header("待完成任務（Inspector 可選指派）")]
    // [SerializeField] private Text missionFlightText;
    // [SerializeField] private Text missionFighterModText;
    // [SerializeField] private Text missionStoryText;

    [SerializeField] private Text[] st, sf;
    [SerializeField] private Image[] sI;

    private bool _langSubscribed;

    private void OnEnable()
    {
        TrySubscribeLanguageChanged();
        RefreshAndLog();
    }

    private void Start()
    {
        TrySubscribeLanguageChanged();
        RefreshUI();
    }

    private void OnDisable()
    {
        UnsubscribeLanguageChanged();
    }

    private void OnDestroy()
    {
        UnsubscribeLanguageChanged();
    }

    private void TrySubscribeLanguageChanged()
    {
        if (_langSubscribed) return;
        if (SetLang.Instance == null) return;
        SetLang.Instance.OnLanguageChanged += OnLanguageChanged;
        _langSubscribed = true;
    }

    private void UnsubscribeLanguageChanged()
    {
        if (!_langSubscribed) return;
        if (SetLang.Instance != null)
            SetLang.Instance.OnLanguageChanged -= OnLanguageChanged;
        _langSubscribed = false;
    }

    private void OnLanguageChanged(Language _)
    {
        RefreshUI();
    }

    /// <summary>重新整理 UI 並印出目前 PlayerPrefs／本地時間對應值。</summary>
    public void RefreshAndLog()
    {
        RefreshUI();
        LogPlayerState();
    }

    /// <summary>依 PlayerPrefs 更新個人資料與任務狀態 UI（不含 Debug.Log）。</summary>
    public void RefreshUI()
    {
        int uid = PlayerPrefs.GetInt("air_museum_uid", 0);
        string pilotName = PlayerPrefs.GetString("air_museum_name", "");
        int gameScore = PlayerPrefs.GetInt(PrefGameScore, 0);
        int landingScore = PlayerPrefs.GetInt(PrefLandingScore, 0);
        int ranking = PlayerPrefs.GetInt(PrefRanking, 0);

        string playerIdDisplay = uid > 0 ? $"PX-{uid:D6}" : "---";
        string pilotDisplay = string.IsNullOrEmpty(pilotName) ? "---" : pilotName;

        string certDateStr = DateTime.Now.ToString("yyyy / MM / dd");

        bool missionFlight = PlayerPrefs.GetInt(PrefMissionFlight, 0) == 1;
        bool missionFighterMod = PlayerPrefs.GetInt(PrefMissionFighterMod, 0) == 1;
        bool missionStory = PlayerPrefs.GetInt(PrefMissionStory, 0) == 1;

        string doneLabel = SetLang.T("mission.status_done");
        string incompleteLabel = SetLang.T("mission.status_incomplete");

        if (missionFlight)
        {
            st[0].color = sDoneColor;
            sf[0].text = doneLabel;
            sf[0].color = sDoneColor;
            sI[0].gameObject.SetActive(true);
        }
        else
        {
            st[0].color = sFailedColor;
            sf[0].text = incompleteLabel;
            sf[0].color = sFailedColor;
            sI[0].gameObject.SetActive(false);
        }

        if (missionFighterMod)
        {
            st[1].color = sDoneColor;
            sf[1].text = doneLabel;
            sf[1].color = sDoneColor;
            sI[1].gameObject.SetActive(true);
        }
        else
        {
            st[1].color = sFailedColor;
            sf[1].text = incompleteLabel;
            sf[1].color = sFailedColor;
            sI[1].gameObject.SetActive(false);
        }

        if (missionStory)
        {
            st[2].color = sDoneColor;
            sf[2].text = doneLabel;
            sf[2].color = sDoneColor;
            sI[2].gameObject.SetActive(true);
        }
        else
        {
            st[2].color = sFailedColor;
            sf[2].text = incompleteLabel;
            sf[2].color = sFailedColor;
            sI[2].gameObject.SetActive(false);
        }

        SetText(playerIdText, playerIdDisplay);
        SetText(pilotNameText, pilotDisplay);
        SetText(gameScoreText, gameScore.ToString("N0", ScoreCulture));
        SetText(landingScoreText, landingScore.ToString("N0", ScoreCulture));
        SetText(rankingText, ranking > 0 ? $"{ranking}" : "---");
        SetText(certificationDateText, certDateStr);
        // SetText(missionFlightText, flightMissionLabel);
        // SetText(missionFighterModText, fighterModLabel);
        // SetText(missionStoryText, storyLabel);
    }

    private void LogPlayerState()
    {
        int uid = PlayerPrefs.GetInt("air_museum_uid", 0);
        string pilotName = PlayerPrefs.GetString("air_museum_name", "");
        int gameScore = PlayerPrefs.GetInt(PrefGameScore, 0);
        int landingScore = PlayerPrefs.GetInt(PrefLandingScore, 0);
        int ranking = PlayerPrefs.GetInt(PrefRanking, 0);
        string certDateStr = DateTime.Now.ToString("yyyy / MM / dd");
        string playerIdDisplay = uid > 0 ? $"PX-{uid:D6}" : "---";
        string pilotDisplay = string.IsNullOrEmpty(pilotName) ? "---" : pilotName;

        Debug.Log(
            "[DisplayPlayerInfo]\n" +
            $"編號(ID)： {playerIdDisplay}\n" +
            $"飛行員名稱： {pilotDisplay}\n" +
            $"遊戲分數 ({PrefGameScore})： {gameScore}\n" +
            $"降落分數 ({PrefLandingScore})： {landingScore}\n" +
            $"飛行排行 ({PrefRanking})： {ranking}\n" +
            $"認證日期（裝置本地時間）： {certDateStr}\n"
        // $"飛行任務 ({PrefMissionFlight}) raw： {missionFlight} → {flightMissionLabel}\n" +
        // $"戰機改造 ({PrefMissionFighterMod}) raw： {missionFighterMod} → {fighterModLabel}\n" +
        // $"飛行故事 ({PrefMissionStory}) raw： {missionStory} → {storyLabel}"
        );
    }

    private static string MissionStatusLabel(int value)
    {
        return value != 0 ? SetLang.T("mission.status_done") : SetLang.T("mission.status_incomplete");
    }

    private static void SetText(Text label, string value)
    {
        if (label != null)
            label.text = value;
    }
}
