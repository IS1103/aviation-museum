// Doll.cs - 拍照＋性別/年齡/眼鏡分析完成後的下一階段。
// 會在 Start 印出前面流程寫入 PlayerPrefs 的資料：
//   PlayerInput 註冊：air_museum_uid / air_museum_name / air_museum_age / air_museum_sex
//   WebCamDisplay 分析：air_museum_face_gender / air_museum_face_age / air_museum_face_glasses_num
// 注意：眼鏡若模型無法判斷，WebCamDisplay 一律存 0（視為沒戴）。
using UnityEngine;
using UnityEngine.UI;

public class Doll : MonoBehaviour
{
    [Header("角色")]
    [Tooltip("要控制的 Man 角色物件")]
    [SerializeField] private Man man;

    [Header("流程按鈕")]
    [Tooltip("確認按鈕：關閉 this 並開啟 menuObj")]
    [SerializeField] private Button confirmButton;
    [Tooltip("自定服裝按鈕：關閉 this 並開啟 dressUpHouseObj")]
    [SerializeField] private Button customClothesButton;

    [Header("流程目標 GameObject")]
    [Tooltip("按下「確認」後要開啟的 Menu GameObject")]
    [SerializeField] private GameObject menuObj;
    [Tooltip("按下「自定服裝」後要開啟的變裝屋 GameObject")]
    [SerializeField] private GameObject dressUpHouseObj;

    private void OnEnable()
    {
        if (confirmButton != null)
            confirmButton.onClick.AddListener(OnConfirmClicked);
        if (customClothesButton != null)
            customClothesButton.onClick.AddListener(OnCustomClothesClicked);
    }

    private void OnDisable()
    {
        if (confirmButton != null)
            confirmButton.onClick.RemoveListener(OnConfirmClicked);
        if (customClothesButton != null)
            customClothesButton.onClick.RemoveListener(OnCustomClothesClicked);
    }

    private void OnConfirmClicked()
    {
        SwitchTo(menuObj);
    }

    private void OnCustomClothesClicked()
    {
        SwitchTo(dressUpHouseObj);
    }

    private void SwitchTo(GameObject target)
    {
        if (target != null)
        {
            target.SetActive(true);
        }
        gameObject.SetActive(false);
    }

    void Start()
    {
        int uid = PlayerPrefs.GetInt("air_museum_uid", 0);
        string name = PlayerPrefs.GetString("air_museum_name", "");
        int regAge = PlayerPrefs.GetInt("air_museum_age", 0);
        int sex = PlayerPrefs.GetInt("air_museum_sex", -1);

        int faceGenderRaw = PlayerPrefs.GetInt("air_museum_face_gender", -1);
        int faceAge = PlayerPrefs.GetInt("air_museum_face_age", -1);
        bool hasGlasses = HasGlasses();

        int eyesIndex = PlayerPrefs.GetInt("air_museum_eyes_index", 0);
        int mouthIndex = PlayerPrefs.GetInt("air_museum_mouth_index", 0);
        int glassesIndex = PlayerPrefs.GetInt("air_museum_glasses_index", 0);
        int helmetIndex = PlayerPrefs.GetInt("air_museum_helmet_index", 0);

        string sexText = SexToText(sex);
        string faceGenderText = faceGenderRaw == 0 ? "Male" : faceGenderRaw == 1 ? "Female" : "未知";
        string glassesText = hasGlasses ? "有戴眼鏡" : "沒戴眼鏡";

        if (man != null)
        {
            man.SetGlassesVisible(hasGlasses);
        }

        Debug.Log(
            "[Doll] PlayerPrefs 狀態：\n" +
            $"  註冊資料  → uid={uid}, name=\"{name}\", age={regAge}, sex={sex}({sexText})\n" +
            $"  臉部分析  → gender={faceGenderRaw}({faceGenderText}), age={faceAge}, glasses={glassesText}\n" +
            $"  模型編號  → eyes={eyesIndex}, mouth={mouthIndex}, glasses={glassesIndex}, helmet={helmetIndex}"
        );
    }

    // 從 PlayerPrefs 讀取臉部分析結果，判斷是否有戴眼鏡（WebCamDisplay 無法判斷時會存 0）。
    public bool HasGlasses()
    {
        return PlayerPrefs.GetInt("air_museum_face_glasses_num", -1) == 1;
    }

    private static string SexToText(int sex)
    {
        switch (sex)
        {
            case 0: return "男";
            case 1: return "女";
            case 2: return "非二元";
            default: return "未選";
        }
    }
}
