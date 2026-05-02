using UnityEngine;
using UnityEngine.UI;

public class FaceChanger : MonoBehaviour
{
    public Renderer targetRenderer;
    public int gridSize = 2;
    public Button[] faceButtons;

    public int faceIndex = 0;//0:眼睛 1:眉毛 2:嘴巴

    private MaterialPropertyBlock block;

    void Start()
    {
        block = new MaterialPropertyBlock();
        SetFace(0);

        for (int i = 0; i < faceButtons.Length; i++)
        {
            int index = i;
            if (faceButtons[i] != null)
            {
                faceButtons[i].onClick.AddListener(() => SetFace(index));
            }
        }
    }

    public void SetFace(int index)
    {
        Debug.Log("SetFace: " + index);

        int x = index % gridSize;
        int y = index / gridSize;

        Vector2 scale = new Vector2(1f, 1f);
        float size = 1f / gridSize;
        Vector2 offset = new Vector2(x * size, y * size);

        targetRenderer.GetPropertyBlock(block);
        block.SetVector("_MainTex_ST", new Vector4(scale.x, scale.y, offset.x, offset.y));
        targetRenderer.SetPropertyBlock(block);

        switch (faceIndex)
        {
            case 0://眼睛
            PlayerPrefs.SetInt("air_museum_eyes_index", index);
                break;
            case 1://眉毛
                PlayerPrefs.SetInt("air_museum_eyebrow_index", index);
                break;
            case 2://嘴巴
                PlayerPrefs.SetInt("air_museum_mouth_index", index);
                break;
            default:
                break;
        }
    }
}