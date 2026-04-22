// Doll.cs - 拍照＋性別/年齡/眼鏡分析完成後的下一階段。
// 會在 Start 印出前面流程寫入 PlayerPrefs 的資料：
//   PlayerInput 註冊：air_museum_uid / air_museum_name / air_museum_age / air_museum_sex
//   WebCamDisplay 分析：air_museum_face_gender / air_museum_face_age / air_museum_face_glasses
// 注意：眼鏡若模型無法判斷，WebCamDisplay 一律存 0（視為沒戴）。
using UnityEngine;

public class Doll : MonoBehaviour
{
    void Start()
    {
        int uid = PlayerPrefs.GetInt("air_museum_uid", 0);
        string name = PlayerPrefs.GetString("air_museum_name", "");
        int regAge = PlayerPrefs.GetInt("air_museum_age", 0);
        int sex = PlayerPrefs.GetInt("air_museum_sex", -1);

        int faceGenderRaw = PlayerPrefs.GetInt("air_museum_face_gender", -1);
        int faceAge = PlayerPrefs.GetInt("air_museum_face_age", -1);
        int glasses = PlayerPrefs.GetInt("air_museum_face_glasses", 0);

        string sexText = SexToText(sex);
        string faceGenderText = faceGenderRaw == 0 ? "Male" : faceGenderRaw == 1 ? "Female" : "未知";
        string glassesText = glasses == 1 ? "有戴眼鏡" : "沒戴眼鏡";

        Debug.Log(
            "[Doll] PlayerPrefs 狀態：\n" +
            $"  註冊資料  → uid={uid}, name=\"{name}\", age={regAge}, sex={sex}({sexText})\n" +
            $"  臉部分析  → gender={faceGenderRaw}({faceGenderText}), age={faceAge}, glasses={glassesText}"
        );
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
