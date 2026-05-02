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

        int eyesIndex = PlayerPrefs.GetInt("air_museum_eyes_index", 0);
        int eyebrowIndex = PlayerPrefs.GetInt("air_museum_eyebrow_index", 0);
        int mouthIndex = PlayerPrefs.GetInt("air_museum_mouth_index", 0);
        int glassesIndex = PlayerPrefs.GetInt("air_museum_glasses_index", -1);
        int helmetIndex = PlayerPrefs.GetInt("air_museum_helmet_index", 0);

        man.SetEyes(eyesIndex);
        man.SetEyebrow(eyebrowIndex);
        man.SetMouth(mouthIndex);
        man.SetGlasses(glassesIndex);
        man.SetHelmet(helmetIndex);

        Debug.Log(
            "[Doll] PlayerPrefs 狀態：\n" +
            $"  註冊資料  → uid={uid}, name=\"{name}\", age={regAge}, sex={sex}\n" +
            $"  模型編號  → eyes={eyesIndex}, mouth={mouthIndex}, glasses={glassesIndex}, helmet={helmetIndex}"
        );
    }
}
