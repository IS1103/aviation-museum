using UnityEngine;
using UnityEngine.EventSystems;

public class UISwipeRotate : MonoBehaviour, IBeginDragHandler, IDragHandler
{
    [Header("旋轉目標")]
    public Transform target;

    [Header("靈敏度")]
    public float sensitivity = 0.2f;

    [Header("是否反轉")]
    public bool invert = false;

    private float lastX;

    public void OnBeginDrag(PointerEventData eventData)
    {
        lastX = eventData.position.x;
    }

    public void OnDrag(PointerEventData eventData)
    {
        float deltaX = eventData.position.x - lastX;
        lastX = eventData.position.x;

        float direction = invert ? -1f : 1f;

        float rotationY = deltaX * sensitivity * direction;

        if (target != null)
        {
            target.Rotate(0, rotationY, 0, Space.World);
        }
    }
}