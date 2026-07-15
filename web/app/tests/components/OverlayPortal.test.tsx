import { render } from "@testing-library/react";
import type { ReactNode } from "react";
import {
  DialogContent,
  DialogRoot,
  DialogTitle,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuRoot,
  DropdownMenuTrigger,
  OverlayPortalProvider,
  PopoverContent,
  PopoverRoot,
  PopoverTrigger,
  Select,
  TooltipContent,
  TooltipRoot,
  TooltipTrigger,
} from "@/components/ui";

type OverlayCase = {
  name: string;
  renderOverlay: () => ReactNode;
  selector: string;
};

const overlayCases: OverlayCase[] = [
  {
    name: "select",
    selector: ".csg-select-content",
    renderOverlay: () => (
      <Select
        open
        value="remote"
        options={[
          { value: "remote", label: "Remote" },
          { value: "local", label: "Local" },
        ]}
      />
    ),
  },
  {
    name: "dropdown menu",
    selector: ".csg-dropdown-menu-content",
    renderOverlay: () => (
      <DropdownMenuRoot open>
        <DropdownMenuTrigger>Open menu</DropdownMenuTrigger>
        <DropdownMenuContent>
          <DropdownMenuItem>Menu item</DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenuRoot>
    ),
  },
  {
    name: "dialog",
    selector: ".csg-dialog-content",
    renderOverlay: () => (
      <DialogRoot open>
        <DialogContent>
          <DialogTitle>Dialog title</DialogTitle>
        </DialogContent>
      </DialogRoot>
    ),
  },
  {
    name: "popover",
    selector: ".csg-popover-content",
    renderOverlay: () => (
      <PopoverRoot open>
        <PopoverTrigger>Open popover</PopoverTrigger>
        <PopoverContent>Popover content</PopoverContent>
      </PopoverRoot>
    ),
  },
  {
    name: "tooltip",
    selector: ".csg-tooltip-content",
    renderOverlay: () => (
      <TooltipRoot open>
        <TooltipTrigger>Tooltip trigger</TooltipTrigger>
        <TooltipContent>Tooltip content</TooltipContent>
      </TooltipRoot>
    ),
  },
];

describe("OverlayPortalProvider", () => {
  it.each(overlayCases)("renders $name content in the shared app overlay root", ({ renderOverlay, selector }) => {
    const { container } = render(<OverlayPortalProvider>{renderOverlay()}</OverlayPortalProvider>);

    const overlayRoot = container.querySelector("[data-csg-overlay-root]");
    const content = document.querySelector(selector);

    expect(overlayRoot).toBeInTheDocument();
    expect(content).toBeInTheDocument();
    expect(overlayRoot).toContainElement(content as HTMLElement);
  });

  it("keeps select content inside a transformed app root in the embedded document", () => {
    const iframe = document.createElement("iframe");
    document.body.appendChild(iframe);

    const frameDocument = iframe.contentDocument;
    expect(frameDocument).not.toBeNull();

    const appRoot = frameDocument!.createElement("div");
    appRoot.id = "embedded-app";
    appRoot.style.transform = "scale(0.9)";
    frameDocument!.body.appendChild(appRoot);

    const view = render(
      <OverlayPortalProvider>
        <Select
          open
          value="remote"
          options={[
            { value: "remote", label: "Remote" },
            { value: "local", label: "Local" },
          ]}
        />
      </OverlayPortalProvider>,
      { baseElement: frameDocument!.body, container: appRoot },
    );

    try {
      const overlayRoot = frameDocument!.querySelector("[data-csg-overlay-root]");
      const content = frameDocument!.querySelector(".csg-select-content");

      expect(overlayRoot?.ownerDocument).toBe(frameDocument);
      expect(content?.ownerDocument).toBe(frameDocument);
      expect(appRoot).toContainElement(overlayRoot as HTMLElement);
      expect(overlayRoot).toContainElement(content as HTMLElement);
      expect(document.body).not.toContainElement(content as HTMLElement);
    } finally {
      view.unmount();
      iframe.remove();
    }
  });

  it("lets a component-specific portal container override the shared target", () => {
    const overrideContainer = document.createElement("div");
    document.body.appendChild(overrideContainer);

    try {
      const { container } = render(
        <OverlayPortalProvider>
          <Select
            contentProps={{ portalContainer: overrideContainer }}
            open
            value="remote"
            options={[
              { value: "remote", label: "Remote" },
              { value: "local", label: "Local" },
            ]}
          />
        </OverlayPortalProvider>,
      );

      const overlayRoot = container.querySelector("[data-csg-overlay-root]");
      const content = document.querySelector(".csg-select-content");

      expect(overrideContainer).toContainElement(content as HTMLElement);
      expect(overlayRoot).not.toContainElement(content as HTMLElement);
    } finally {
      overrideContainer.remove();
    }
  });
});
