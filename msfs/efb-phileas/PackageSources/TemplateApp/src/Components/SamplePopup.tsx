import { TTButton, GamepadUiView, RequiredProps, TVNode, UiViewProps } from "@efb/efb-api";
import { FSComponent } from "@microsoft/msfs-sdk";
import "./SamplePopup.scss";

export class SamplePopup extends GamepadUiView<HTMLDivElement, RequiredProps<UiViewProps, "appViewService">> {
  public readonly tabName = SamplePopup.name;

  public render(): TVNode<HTMLDivElement> {
    return (
      <div ref={this.gamepadUiViewRef} class="sample-popup">
        <TTButton
          key="CLOSE"
          type="primary"
          callback={(): void => {
            this.props.appViewService.goBack();
          }}
        />
        <div class="content">
          Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed nec diam euismod, aliquam mi nec
        </div>
      </div>
    );
  }
}
