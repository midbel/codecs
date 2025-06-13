<?xml version="1.0" encoding="UTF-8"?>

<xsl:stylesheet version="3.0" 
	xmlns:xsl="http://www.w3.org/1999/XSL/Transform">
	<xsl:output method="xml" indent="yes"/>
	<xsl:template match="/">
		<if>
			<xsl:if test="false()">
				<item>
					<result test="false"><xsl:value-of select="/root/item"/></result>
				</item>
			</xsl:if>
			<xsl:if test="not(())">
				<item>
					<result test="empty"><xsl:value-of select="/root/item"/></result>
				</item>
			</xsl:if>
		</if>
	</xsl:template>
</xsl:stylesheet>