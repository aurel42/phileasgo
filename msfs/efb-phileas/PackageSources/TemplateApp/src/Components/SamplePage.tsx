import { TTButton, GamepadUiView, RequiredProps, TVNode, UiViewProps } from "@efb/efb-api";
import { FSComponent } from "@microsoft/msfs-sdk";
import "./SamplePage.scss";

interface SamplePageProps extends RequiredProps<UiViewProps, "appViewService"> {
  /** The page title */
  title: string;

  /** The page background color */
  color: string;
}

export class SamplePage extends GamepadUiView<HTMLDivElement, SamplePageProps> {
  public readonly tabName = SamplePage.name;

  public render(): TVNode<HTMLDivElement> {
    return (
      <div ref={this.gamepadUiViewRef} class="sample-page" style={`--color: ${this.props.color}`}>
        <div class="header">
          <TTButton
            key="Go back"
            type="secondary"
            callback={(): void => {
              this.props.appViewService.goBack();
            }}
          />
          <h2>{this.props.title}</h2>
        </div>

        <div class="content">
          <TTButton
            key="Open page 1"
            callback={(): void => {
              console.log("SamplePage1");
              this.props.appViewService.open("SamplePage1");
            }}
          />
          <TTButton
            key="Open page 2"
            callback={(): void => {
              console.log("SamplePage2");
              this.props.appViewService.open("SamplePage2");
            }}
          />
          <TTButton
            key="Open popup"
            callback={(): void => {
              console.log("SamplePopup");
              this.props.appViewService.open("SamplePopup");
            }}
          />
        </div>
      </div>
    );
  }
}
