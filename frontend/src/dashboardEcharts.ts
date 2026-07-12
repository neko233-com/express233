import * as echarts from "echarts/core";
import { BarChart, LineChart } from "echarts/charts";
import { GridComponent, LegendComponent, TooltipComponent } from "echarts/components";
import { SVGRenderer } from "echarts/renderers";

// Keep the dashboard payload scoped to the two operational chart types.
echarts.use([BarChart, GridComponent, LegendComponent, LineChart, SVGRenderer, TooltipComponent]);

export { echarts };
